package argo

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestCountsAndAttention(t *testing.T) {
	apps := []App{
		{Name: "ok", Sync: "Synced", Health: "Healthy"},
		{Name: "drift", Sync: "OutOfSync", Health: "Healthy"},
		{Name: "bad", Sync: "OutOfSync", Health: "Degraded"},
		{Name: "gone", Sync: "Synced", Health: "Missing"},
		{Name: "wait", Sync: "Synced", Health: "Progressing"},
		{Name: "blank", Sync: "", Health: ""},
		{Name: "held", Sync: "Synced", Health: "Suspended"},
	}
	stats := Counts(apps)
	if stats.Total != 7 || stats.Synced != 4 || stats.Healthy != 2 || stats.OutOfSync != 2 || stats.Degraded != 1 || stats.Missing != 1 || stats.Progressing != 1 {
		t.Fatalf("%+v", stats)
	}
	rows := NeedsAttention(apps)
	if len(rows) != 3 || rows[0].Reason != "Degraded" || rows[1].Reason != "Missing" || rows[2].Reason != "Out of sync" {
		t.Fatalf("%+v", rows)
	}
	if ShortRevision("abcdef1234567890") != "abcdef1" || ShortRevision("main") != "main" {
		t.Fatal("revision")
	}
	if Category("Warning", "Synced") != "failure" || Category("Normal", "OperationStarted") != "progress" || Category("Normal", "ResourceUpdated") != "success" {
		t.Fatal("category")
	}
}

func TestDrawDashboard(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	page := Page{
		ID:      "argo",
		Context: "dev",
		Now:     now,
		Apps: []App{
			{Namespace: "argo", Name: "shop", Sync: "OutOfSync", Health: "Degraded"},
			{Namespace: "argo", Name: "web", Sync: "Synced", Health: "Healthy"},
		},
		Sets:     []Set{{Name: "shops"}},
		Projects: []Project{{Name: "default"}},
		Activity: []Activity{{Reason: "ComparisonError", Object: "Application argo/shop", Body: "denied", Category: "failure", When: now.Add(-2 * time.Minute)}},
	}
	texts := frame(t, page)
	for _, want := range []string{"Argo", "Synced", "Out of sync", "Degraded", "Needs attention", "shop", "Recent activity", "ComparisonError", "All Applications are synced and healthy."} {
		if want == "All Applications are synced and healthy." {
			if has(texts, want) {
				t.Fatalf("healthy copy shown with a degraded app: %v", texts)
			}
			continue
		}
		if !has(texts, want) {
			t.Fatalf("missing %q in %v", want, texts)
		}
	}
	page.Apps = nil
	texts = frame(t, page)
	if !has(texts, "All Applications are synced and healthy.") {
		t.Fatalf("%v", texts)
	}
	page.Notice = "CRD is not installed"
	texts = frame(t, page)
	if !has(texts, "CRD is not installed") || has(texts, "Install") {
		t.Fatalf("%v", texts)
	}
}

func TestDrawApplicationDetail(t *testing.T) {
	selected := 0
	prune := false
	cascade := "foreground"
	syncRev := ""
	rollback := "2"
	page := Page{
		ID:       "argo-applications",
		Context:  "dev",
		Selected: &selected,
		Prune:    &prune,
		Cascade:  &cascade,
		SyncRev:  &syncRev,
		Revision: &rollback,
		Apps: []App{{
			Namespace:   "argo",
			Name:        "shop",
			Project:     "default",
			Sync:        "OutOfSync",
			Health:      "Degraded",
			Destination: "in-cluster/shop",
			Revision:    "abcdef1234567890",
			Policy:      "Automated, prune",
			Source:      "https://git.example/shop deploy main",
			Resources:   []Resource{{Kind: "Deployment", Name: "web", Sync: "OutOfSync", Health: "Degraded"}},
			History:     []History{{ID: 4, Revision: "bbbbbbbbbbbb", By: "nakube"}},
		}},
	}
	texts := frame(t, page)
	for _, want := range []string{"shop", "in-cluster/shop", "abcdef1", "Automated, prune", "Deployment", "Sync", "Hard refresh", "Foreground", "Prune"} {
		if !has(texts, want) {
			t.Fatalf("missing %q in %v", want, texts)
		}
	}
}

func frame(t *testing.T, page Page) []string {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, c.Theme(), page)
	}, 1280, 980)
	return tt.Texts()
}

func has(texts []string, want string) bool {
	for _, text := range texts {
		if text == want || strings.Contains(text, want) {
			return true
		}
	}
	return false
}

func TestDrawLoadingSkeleton(t *testing.T) {
	texts := frame(t, Page{ID: "argo", Context: "dev", Loading: true})
	for _, want := range []string{"Argo", "Applications", "Needs attention", "Recent activity", "Application sets"} {
		if !has(texts, want) {
			t.Errorf("missing %q in %v", want, texts)
		}
	}
	if has(texts, "0 applications") || has(texts, "No applications") || has(texts, "All Applications are synced and healthy.") {
		t.Fatalf("loading drew settled copy %v", texts)
	}
	list := frame(t, Page{ID: "argo-applications", Title: "Applications", Loading: true})
	if !has(list, "Sync") || !has(list, "Health") || has(list, "No applications") {
		t.Fatalf("application skeleton %v", list)
	}
}

func TestDrawPartialArgo(t *testing.T) {
	texts := frame(t, Page{
		ID:          "argo",
		Context:     "dev",
		AppsLoading: false,
		SetsLoading: true,
		Apps:        []App{{Name: "shop", Namespace: "shop", Sync: "Synced", Health: "Healthy"}},
	})
	if !has(texts, "Applications") || !has(texts, "1") {
		t.Fatalf("arrived apps were hidden: %v", texts)
	}
	if has(texts, "0 applications") {
		t.Fatalf("partial load invented an empty fleet: %v", texts)
	}
}
