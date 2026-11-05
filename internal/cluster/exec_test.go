package cluster

import (
	"context"
	"io"
	"testing"
)

func TestExecConnResizeRecordsColumns(t *testing.T) {
	conn, stdinR, stdoutW := newExecConn(func() {})
	defer stdinR.Close()
	defer stdoutW.Close()
	defer conn.Close()

	if err := conn.Resize(132, 43); err != nil {
		t.Fatal(err)
	}
	cols, rows := conn.Size()
	if cols != 132 || rows != 43 {
		t.Fatalf("size = %d×%d", cols, rows)
	}

	if _, err := conn.Write([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 2)
	if _, err := io.ReadFull(stdinR, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "hi" {
		t.Fatalf("stdin = %q", buf)
	}

	if _, err := stdoutW.Write([]byte("ok")); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "ok" {
		t.Fatalf("stdout = %q", buf)
	}
}

func TestExecConnCloseCancels(t *testing.T) {
	cancelled := false
	conn, stdinR, stdoutW := newExecConn(func() { cancelled = true })
	defer stdinR.Close()
	defer stdoutW.Close()
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if !cancelled {
		t.Fatal("Close did not cancel the remote command")
	}
	if _, err := conn.Write([]byte("x")); err == nil {
		t.Fatal("write after close succeeded")
	}
}

func TestExecRejectsEmptyCommand(t *testing.T) {
	s := NewSession()
	if _, err := s.Exec(context.Background(), "ns", "pod", "c", nil); err == nil {
		t.Fatal("nil command accepted")
	}
	if _, err := s.Exec(context.Background(), "ns", "pod", "c", []string{""}); err == nil {
		t.Fatal("empty command accepted")
	}
}
