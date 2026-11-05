package desktop

import "testing"

func TestMissingCRDNotice(t *testing.T) {
	if emptyText("port-forwards") == "None" {
		t.Fatal("port-forward empty text is the generic one")
	}
	if emptyText("services") != "None" {
		t.Fatalf("services empty = %q", emptyText("services"))
	}
}
