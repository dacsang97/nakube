package cluster

import (
	"context"
	"strings"
	"testing"

	"k8s.io/client-go/rest"
)

func TestHelmDryRunDoesNotInstall(t *testing.T) {
	called := false
	prev := helmMutate
	helmMutate = func(HelmRequest) error {
		called = true
		return nil
	}
	t.Cleanup(func() { helmMutate = prev })

	s := NewSession()
	res, err := s.HelmInstall(context.Background(), HelmRequest{
		Release:   "demo",
		Namespace: "ns",
		Chart:     "testdata/charts/demo",
		Values:    "note: from-test\n",
		DryRun:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("dry-run invoked the mutating install")
	}
	if res == nil || !strings.Contains(res.Manifest, "ConfigMap") || !strings.Contains(res.Manifest, "from-test") {
		t.Fatalf("manifest = %q", res)
	}
}

func TestHelmGetterUsesSessionContext(t *testing.T) {
	s := &Session{
		name: "B",
		cfg:  &rest.Config{Host: "https://b.example"},
	}
	g, err := s.HelmGetter("team")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := g.ToRESTConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "https://b.example" {
		t.Fatalf("host = %s", cfg.Host)
	}
	raw, err := g.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		t.Fatal(err)
	}
	if raw.CurrentContext != "B" {
		t.Fatalf("loader context = %q", raw.CurrentContext)
	}
	if raw.Contexts["B"].Namespace != "team" {
		t.Fatalf("namespace = %q", raw.Contexts["B"].Namespace)
	}
}
