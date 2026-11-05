package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dacsang97/nakube/internal/cluster"
)
// API is the cluster surface MCP may call. It has no apply, delete, or scale.
type API interface {
	ListGeneric(ctx context.Context, gvrGroup, version, resource, namespace string) ([]cluster.GenericRow, error)
	GetYAML(ctx context.Context, kind, namespace, name string) (string, error)
	StreamLogs(ctx context.Context, namespace, pod, container string, follow bool, tail int64, onBatch func([]string)) error
	Exec(ctx context.Context, namespace, pod, container string, command []string) (io.ReadWriteCloser, error)
}

// SessionAPI adapts the one cluster session. It does not add a write method.
type SessionAPI struct{ S *cluster.Session }

func (a SessionAPI) ListGeneric(ctx context.Context, group, version, resource, namespace string) ([]cluster.GenericRow, error) {
	return a.S.ListGeneric(ctx, schema.GroupVersionResource{Group: group, Version: version, Resource: resource}, namespace)
}

func (a SessionAPI) GetYAML(ctx context.Context, kind, namespace, name string) (string, error) {
	return a.S.GetYAML(ctx, cluster.Identity{Kind: kind, Namespace: namespace, Name: name})
}

func (a SessionAPI) StreamLogs(ctx context.Context, namespace, pod, container string, follow bool, tail int64, onBatch func([]string)) error {
	return a.S.StreamLogs(ctx, namespace, pod, container, follow, tail, onBatch)
}

func (a SessionAPI) Exec(ctx context.Context, namespace, pod, container string, command []string) (io.ReadWriteCloser, error) {
	return a.S.Exec(ctx, namespace, pod, container, command)
}

// ToolNames are the only tools. Write-shaped names are unknown.
var ToolNames = []string{"list_resources", "get_resource", "stream_logs", "exec"}

// Server is a stdio MCP process. It does not listen on a port.
type Server struct {
	API API
}

// Serve reads JSON-RPC frames from in and writes frames to out until in closes.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)
	for {
		msg, err := readFrame(r)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := s.handle(ctx, msg, out); err != nil {
			return err
		}
	}
}

type rpc struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (s *Server) handle(ctx context.Context, msg rpc, out io.Writer) error {
	switch msg.Method {
	case "initialize":
		return writeResult(out, msg.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "nakube", "version": "0.1.0"},
		})
	case "notifications/initialized", "initialized":
		return nil
	case "tools/list":
		tools := make([]map[string]any, 0, len(ToolNames))
		for _, name := range ToolNames {
			tools = append(tools, map[string]any{"name": name, "description": name, "inputSchema": map[string]any{"type": "object"}})
		}
		return writeResult(out, msg.ID, map[string]any{"tools": tools})
	case "tools/call":
		return s.call(ctx, msg, out)
	default:
		if msg.ID == nil {
			return nil
		}
		return writeError(out, msg.ID, -32601, "unknown method")
	}
}

func (s *Server) call(ctx context.Context, msg rpc, out io.Writer) error {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if len(msg.Params) > 0 {
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return writeError(out, msg.ID, -32602, "invalid params")
		}
	}
	switch params.Name {
	case "list_resources", "get_resource", "stream_logs", "exec":
	default:
		return writeError(out, msg.ID, -32602, "unknown tool")
	}
	if s.API == nil {
		return writeError(out, msg.ID, -32000, "no kube context is open")
	}
	switch params.Name {
	case "list_resources":
		rows, err := s.API.ListGeneric(ctx, str(params.Arguments, "group"), str(params.Arguments, "version"), str(params.Arguments, "resource"), str(params.Arguments, "namespace"))
		if err != nil {
			return writeError(out, msg.ID, -32000, err.Error())
		}
		return writeResult(out, msg.ID, toolText(mustJSON(rows)))
	case "get_resource":
		body, err := s.API.GetYAML(ctx, str(params.Arguments, "kind"), str(params.Arguments, "namespace"), str(params.Arguments, "name"))
		if err != nil {
			return writeError(out, msg.ID, -32000, err.Error())
		}
		return writeResult(out, msg.ID, toolText(body))
	case "stream_logs":
		token := progressToken(msg)
		n := 0
		err := s.API.StreamLogs(ctx, str(params.Arguments, "namespace"), str(params.Arguments, "pod"), str(params.Arguments, "container"), true, int64Arg(params.Arguments, "tail"), func(lines []string) {
			n++
			_ = writeNote(out, "notifications/progress", map[string]any{
				"progressToken": token,
				"progress":      n,
				"message":       strings.Join(lines, "\n"),
			})
		})
		if err != nil {
			return writeError(out, msg.ID, -32000, err.Error())
		}
		return writeResult(out, msg.ID, toolText("log stream closed"))
	case "exec":
		argv := stringSlice(params.Arguments["command"])
		if len(argv) == 0 || argv[0] == "" {
			return writeError(out, msg.ID, -32602, "exec command is required")
		}
		conn, err := s.API.Exec(ctx, str(params.Arguments, "namespace"), str(params.Arguments, "pod"), str(params.Arguments, "container"), argv)
		if err != nil {
			return writeError(out, msg.ID, -32000, err.Error())
		}
		defer conn.Close()
		if input := str(params.Arguments, "input"); input != "" {
			if _, err := io.WriteString(conn, input); err != nil {
				return writeError(out, msg.ID, -32000, err.Error())
			}
		}
		token := progressToken(msg)
		buf := make([]byte, 4096)
		n := 0
		for {
			read, err := conn.Read(buf)
			if read > 0 {
				n++
				_ = writeNote(out, "notifications/progress", map[string]any{
					"progressToken": token,
					"progress":      n,
					"message":       string(buf[:read]),
				})
			}
			if err != nil {
				break
			}
		}
		return writeResult(out, msg.ID, toolText("exec closed"))
	default:
		return writeError(out, msg.ID, -32602, "unknown tool")
	}
}

func progressToken(msg rpc) any {
	var params struct {
		Meta struct {
			ProgressToken any `json:"progressToken"`
		} `json:"_meta"`
	}
	_ = json.Unmarshal(msg.Params, &params)
	if params.Meta.ProgressToken != nil {
		return params.Meta.ProgressToken
	}
	return msg.ID
}

func toolText(s string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": s}}}
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func str(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	default:
		return ""
	}
}

func int64Arg(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	default:
		return 0
	}
}

func stringSlice(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s, ok := item.(string)
			if !ok {
				return nil
			}
			out = append(out, s)
		}
		return out
	case []string:
		return t
	default:
		return nil
	}
}

func writeResult(out io.Writer, id any, result any) error {
	return writeFrame(out, map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func writeError(out io.Writer, id any, code int, message string) error {
	return writeFrame(out, map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}})
}

func writeNote(out io.Writer, method string, params any) error {
	return writeFrame(out, map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func writeFrame(out io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Content-Length: %d\r\n\r\n", len(b)); err != nil {
		return err
	}
	_, err = out.Write(b)
	return err
}

func readFrame(r *bufio.Reader) (rpc, error) {
	var length int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return rpc{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			n, err := strconv.Atoi(strings.TrimSpace(line[len("content-length:"):]))
			if err != nil {
				return rpc{}, err
			}
			length = n
		}
	}
	if length < 0 || length > 8<<20 {
		return rpc{}, errors.New("invalid frame")
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return rpc{}, err
	}
	var msg rpc
	if err := json.Unmarshal(buf, &msg); err != nil {
		return rpc{}, err
	}
	return msg, nil
}

// Frames splits a buffer of MCP frames. Tests use it to assert notification order.
func Frames(b []byte) ([]rpc, error) {
	var out []rpc
	r := bufio.NewReader(bytes.NewReader(b))
	for {
		msg, err := readFrame(r)
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out = append(out, msg)
	}
}
