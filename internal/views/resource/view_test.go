package resource

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestLoadingDrawsSkeleton(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, c.Theme(), List{
			Title:    "Pods",
			Loading:  true,
			Columns:  []string{"Namespace", "Name", "Phase"},
			Selected: new(int),
		})
	}, 800, 600)
	if !tt.HasText("Pods") || !tt.HasText("Namespace") || !tt.HasText("Phase") {
		t.Fatalf("skeleton lost the table chrome %v", tt.Texts())
	}
	if tt.HasText("None") {
		t.Fatalf("loading list drew the empty state %v", tt.Texts())
	}
}

func TestEmptyStaysEmpty(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, c.Theme(), List{Title: "Pods", Empty: "None"})
	}, 800, 600)
	if !tt.HasText("None") {
		t.Fatal("settled empty list hid its copy")
	}
}

func TestHeaderActionsAreSeparatedFromBodyActions(t *testing.T) {
	edit, remove := HeaderActions("Pod")
	if edit != "yaml" || remove != "delete" {
		t.Fatalf("pod header actions = %q, %q", edit, remove)
	}
	if got := BodyActions("Pod"); !slices.Equal(got, []string{"logs", "exec", "forward"}) {
		t.Fatalf("pod body actions = %v", got)
	}
	edit, remove = HeaderActions("Release")
	if edit != "" || remove != "uninstall" {
		t.Fatalf("release header actions = %q, %q", edit, remove)
	}
	if got := BodyActions("Release"); !slices.Equal(got, []string{"rollback"}) {
		t.Fatalf("release body actions = %v", got)
	}
}
