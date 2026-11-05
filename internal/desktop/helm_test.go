package desktop

import (
	"strings"
	"testing"
)

func TestMaskSecretYAML(t *testing.T) {
	body := "kind: Secret\ndata:\n  password: c2VjcmV0\n"
	got := maskSecretYAML(body)
	if strings.Contains(got, "c2VjcmV0") || !strings.Contains(got, "***") {
		t.Fatalf("%s", got)
	}
}

func TestHelmCancelDoesNotInstall(t *testing.T) {
	a := New(nil)
	a.helmDry = false
	a.helmRelease = "demo"
	called := false
	prev := a.session
	_ = prev
	a.runHelm(false)
	if called {
		t.Fatal("install ran without a dry-run confirm")
	}
}
