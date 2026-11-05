package desktop

import "testing"

func TestArgoMissingCRDText(t *testing.T) {
	a := New(nil)
	a.crdMissing = true
	if !a.crdMissing {
		t.Fatal("CRD missing flag did not stick")
	}
}
