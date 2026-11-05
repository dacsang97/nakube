package cluster

import (
	"strings"
	"testing"
)

func TestBuildArgoSyncPatch(t *testing.T) {
	data, err := BuildArgoSyncPatch(ArgoSyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, `"dryRun":false`) {
		t.Fatalf("default patch missing dryRun false: %s", got)
	}
	if !strings.Contains(got, `"revision":"HEAD"`) || !strings.Contains(got, `"hook":{}`) {
		t.Fatalf("empty revision/strategy not defaulted: %s", got)
	}
	dry, err := BuildArgoSyncPatch(ArgoSyncOptions{DryRun: true, Revision: "v1", Strategy: "apply"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dry), `"dryRun":true`) {
		t.Fatalf("dry-run patch: %s", dry)
	}
}

func TestArgoCascade(t *testing.T) {
	if _, keep, err := argoCascade(""); err != nil || !keep {
		t.Fatalf("default cascade keep=%v err=%v", keep, err)
	}
	if _, keep, err := argoCascade("non-cascading"); err != nil || keep {
		t.Fatalf("non-cascading keep=%v err=%v", keep, err)
	}
	if _, _, err := argoCascade("nope"); err == nil {
		t.Fatal("invalid cascade accepted")
	}
}
