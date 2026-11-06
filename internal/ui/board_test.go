package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestAge(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	if Age(time.Time{}, now) != "" {
		t.Fatal("zero time drew an age")
	}
	if Age(now.Add(time.Minute), now) != "0s" {
		t.Fatal("future time drew a negative age")
	}
	if Age(now.Add(-5*time.Second), now) != "5s" {
		t.Fatal("seconds")
	}
	if Age(now.Add(-2*time.Minute), now) != "2m" {
		t.Fatal("minutes")
	}
	if Age(now.Add(-3*time.Hour), now) != "3h" {
		t.Fatal("hours")
	}
	if Age(now.Add(-48*time.Hour), now) != "2d" {
		t.Fatal("days")
	}
	if When(time.Time{}, now) != "" {
		t.Fatal("zero time drew a clock")
	}
	if got := When(now.Add(-2*time.Minute), now); len(got) < 4 || got[:2] != "2m" {
		t.Fatalf("when = %q", got)
	}
}

func TestMatchAndCount(t *testing.T) {
	if !Match("  ", "Ready") || !Match("rea", "Ready") || Match("disk", "Ready") {
		t.Fatal("match")
	}
	if Count(3, 3) != "3" || Count(1, 3) != "1 of 3" {
		t.Fatal("count")
	}
}

func TestPlaceKeepsTheRow(t *testing.T) {
	keys := []string{"a", "b"}
	if Place(0, "b", keys) != 1 {
		t.Fatal("key did not follow the row")
	}
	if Place(0, "gone", keys) != -1 {
		t.Fatal("hidden row stayed selected")
	}
	if Place(4, "", keys) != 0 {
		t.Fatal("first open did not select a row")
	}
	if Place(0, "", nil) != -1 {
		t.Fatal("empty list selected a row")
	}
}

func TestBoardLoadingSkipsEmpty(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		DrawBoard(c, c.Theme(), Board{
			Title:   "Nodes",
			Loading: true,
			Empty:   "None",
			Columns: []Column{{Title: "Name"}, {Title: "Status"}},
		})
	}, 800, 600)
	if !tt.HasText("Nodes") || !tt.HasText("Name") || !tt.HasText("Status") {
		t.Fatalf("skeleton lost the chrome %v", tt.Texts())
	}
	if tt.HasText("None") {
		t.Fatal("loading board drew the empty state")
	}
}

func TestBoardEmpty(t *testing.T) {
	query := ""
	tt := ui.NewTester(func(c *ui.Context) {
		DrawBoard(c, c.Theme(), Board{
			Title: "Nodes",
			Empty: "None",
			Query: &query,
		})
	}, 800, 600)
	if !tt.HasText("None") || !tt.HasText("Nodes") {
		t.Fatalf("empty board %v", tt.Texts())
	}
}
