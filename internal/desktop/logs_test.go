package desktop

import "testing"

func TestPodColorIsStable(t *testing.T) {
	first := PodColor("web-0")
	second := PodColor("web-0")
	if first != second {
		t.Fatal("pod color changed")
	}
	if PodColor("web-0") == PodColor("web-1") && PodColor("api-0") == PodColor("web-0") {
		t.Fatal("every pod got the same color")
	}
}

func TestFilterLines(t *testing.T) {
	lines := []LogLine{{Pod: "web-0", Text: "ready"}, {Pod: "web-1", Text: "error"}}
	got, err := FilterLines(lines, "error")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Pod != "web-1" {
		t.Fatalf("%+v", got)
	}
	lines[1].Text = "changed"
	if got[0].Text != "error" {
		t.Fatal("filter reused the caller's line")
	}
}
