package mcp

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/dacsang97/nakube/internal/cluster"
)

type fakeAPI struct {
	exec *fakeConn
}

func (f *fakeAPI) ListGeneric(context.Context, string, string, string, string) ([]cluster.GenericRow, error) {
	return []cluster.GenericRow{{Name: "web", Namespace: "ns"}}, nil
}

func (f *fakeAPI) GetYAML(context.Context, string, string, string) (string, error) {
	return "kind: Pod\n", nil
}

func (f *fakeAPI) StreamLogs(_ context.Context, _, _, _ string, _ bool, _ int64, onBatch func([]string)) error {
	onBatch([]string{"one"})
	onBatch([]string{"two"})
	return nil
}

func (f *fakeAPI) Exec(context.Context, string, string, string, []string) (io.ReadWriteCloser, error) {
	f.exec = &fakeConn{read: strings.NewReader("ok")}
	return f.exec, nil
}

type fakeConn struct {
	read   io.Reader
	wrote  bytes.Buffer
	closed bool
}

func (f *fakeConn) Read(p []byte) (int, error)  { return f.read.Read(p) }
func (f *fakeConn) Write(p []byte) (int, error) { return f.wrote.Write(p) }
func (f *fakeConn) Close() error                { f.closed = true; return nil }

func TestToolsAndWriteRejection(t *testing.T) {
	var out bytes.Buffer
	s := &Server{API: &fakeAPI{}}
	in := frames(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"apply","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"delete","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"exec","arguments":{"command":[]}}}`,
	)
	if err := s.Serve(context.Background(), strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	if _, err := Frames(out.Bytes()); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	for _, name := range ToolNames {
		if !strings.Contains(body, name) {
			t.Fatalf("missing tool %s", name)
		}
	}
	if strings.Contains(body, `"name":"apply"`) {
		t.Fatal("listed a write tool")
	}
	if strings.Count(body, "unknown tool") < 2 || !strings.Contains(body, "exec command is required") {
		t.Fatalf("write and empty exec were not all rejected: %s", body)
	}
}

func TestStreamLogsProgressBeforeResult(t *testing.T) {
	var out bytes.Buffer
	s := &Server{API: &fakeAPI{}}
	in := frames(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"stream_logs","arguments":{"namespace":"ns","pod":"web","container":"app"},"_meta":{"progressToken":"t"}}}`)
	if err := s.Serve(context.Background(), strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	msgs, err := Frames(out.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	notes := 0
	sawResult := false
	for _, msg := range msgs {
		if msg.Method == "notifications/progress" {
			if sawResult {
				t.Fatal("progress after the final result")
			}
			notes++
		}
		if msg.ID != nil && msg.Method == "" {
			sawResult = true
		}
	}
	if notes < 2 {
		t.Fatalf("progress notes = %d", notes)
	}
	if !sawResult {
		t.Fatal("missing final result")
	}
}

func TestExecWritesInput(t *testing.T) {
	api := &fakeAPI{}
	var out bytes.Buffer
	s := &Server{API: api}
	in := frames(`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"exec","arguments":{"namespace":"ns","pod":"web","container":"app","command":["/bin/sh"],"input":"pwd\n"}}}`)
	if err := s.Serve(context.Background(), strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	if api.exec == nil || api.exec.wrote.String() != "pwd\n" {
		t.Fatalf("exec write = %#v", api.exec)
	}
	msgs, err := Frames(out.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) < 2 || msgs[0].Method != "notifications/progress" {
		t.Fatalf("expected progress before result, got %+v", msgs)
	}
}

func frames(bodies ...string) string {
	var b strings.Builder
	for _, body := range bodies {
		b.WriteString("Content-Length: ")
		b.WriteString(itoa(len(body)))
		b.WriteString("\r\n\r\n")
		b.WriteString(body)
	}
	return b.String()
}

func itoa(n int) string { return strconv.Itoa(n) }
