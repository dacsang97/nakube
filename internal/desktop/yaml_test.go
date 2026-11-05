package desktop_test

import (
	"errors"
	"testing"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/desktop"
)

func TestCancelDoesNotApply(t *testing.T) {
	preview := &cluster.ApplyPreview{
		Identity:  cluster.Identity{Kind: "ConfigMap", Namespace: "ns", Name: "demo"},
		Live:      "k: old\n",
		Predicted: "k: v\n",
	}
	dialog := desktop.OpenYAML(preview, "kind: ConfigMap\n")
	if dialog.Live != "k: old\n" || dialog.Predicted != "k: v\n" {
		t.Fatalf("diff state = %+v", dialog)
	}
	called := false
	err := desktop.ConfirmYAML(dialog, false, func(cluster.Identity, string) error {
		called = true
		return errors.New("apply invoked")
	})
	if err != nil || called {
		t.Fatalf("cancel called apply: %v %v", called, err)
	}
	if err := desktop.ConfirmYAML(dialog, true, func(id cluster.Identity, body string) error {
		if id.Name != "demo" || body == "" {
			t.Fatalf("confirm lost identity %+v", id)
		}
		called = true
		return nil
	}); err != nil || !called {
		t.Fatalf("confirm did not apply: %v %v", called, err)
	}
}
