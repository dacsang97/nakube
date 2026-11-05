package argo

import (
	"strings"
	"time"
	"unicode"
)

// App is one Application row. Empty sync or health is not a status.
type App struct {
	Namespace        string
	Name             string
	Project          string
	Sync             string
	Health           string
	HealthMessage    string
	Revision         string
	TargetRevision   string
	Destination      string
	Policy           string
	Source           string
	Resources        []Resource
	History          []History
	Conditions       []string
	OperationPhase   string
	OperationMessage string
	OperationBy      string
	Images           []string
	SourceType       string
}

// Resource is one managed object on an Application.
type Resource struct {
	Kind      string
	Namespace string
	Name      string
	Sync      string
	Health    string
	Hook      bool
}

// History is one past deploy, newest first when the cluster parser sorted it.
type History struct {
	ID       int64
	Revision string
	By       string
}

// Set is one ApplicationSet row.
type Set struct {
	Namespace   string
	Name        string
	Generators  string
	Project     string
	Destination string
	Conditions  []string
}

// Project is one AppProject row.
type Project struct {
	Namespace    string
	Name         string
	Description  string
	SourceRepos  []string
	Destinations []string
	Roles        int
}

// Repo is one repository Secret. It has no credentials.
type Repo struct {
	Namespace string
	Name      string
	Type      string
	URL       string
	Project   string
}

// Cluster is one cluster Secret. It has no credentials.
type Cluster struct {
	Namespace string
	Name      string
	Server    string
}

// Activity is one Argo event, newest first.
type Activity struct {
	Reason   string
	Object   string
	Body     string
	Category string
	When     time.Time
}

// Stats are the dashboard counts. Unknown and Suspended are not buckets.
type Stats struct {
	Total       int
	Synced      int
	Healthy     int
	OutOfSync   int
	Degraded    int
	Missing     int
	Progressing int
}

// Attention is an Application that is degraded, missing, or out of sync.
type Attention struct {
	Namespace string
	Name      string
	Sync      string
	Health    string
	Reason    string
}

// Counts rolls up sync and health. An empty status increments neither side.
func Counts(apps []App) Stats {
	var s Stats
	s.Total = len(apps)
	for _, app := range apps {
		switch app.Sync {
		case "Synced":
			s.Synced++
		case "OutOfSync":
			s.OutOfSync++
		}
		switch app.Health {
		case "Healthy":
			s.Healthy++
		case "Degraded":
			s.Degraded++
		case "Missing":
			s.Missing++
		case "Progressing":
			s.Progressing++
		}
	}
	return s
}

// NeedsAttention returns degraded, then missing, then out of sync.
// Progressing alone is not a problem. Health outranks sync.
func NeedsAttention(apps []App) []Attention {
	var degraded, missing, drift []Attention
	for _, app := range apps {
		item := Attention{
			Namespace: app.Namespace,
			Name:      app.Name,
			Sync:      shown(app.Sync),
			Health:    shown(app.Health),
		}
		switch app.Health {
		case "Degraded":
			item.Reason = "Degraded"
			degraded = append(degraded, item)
		case "Missing":
			item.Reason = "Missing"
			missing = append(missing, item)
		default:
			if app.Sync == "OutOfSync" {
				item.Reason = "Out of sync"
				drift = append(drift, item)
			}
		}
	}
	out := make([]Attention, 0, len(degraded)+len(missing)+len(drift))
	out = append(out, degraded...)
	out = append(out, missing...)
	out = append(out, drift...)
	return out
}

// Category labels an event the way the Argo feed does.
// Warning, or a reason containing error or fail, is a failure.
func Category(eventType, reason string) string {
	if strings.EqualFold(eventType, "Warning") || containsFold(reason, "error") || containsFold(reason, "fail") {
		return "failure"
	}
	if containsFold(reason, "start") || containsFold(reason, "progress") || containsFold(reason, "running") {
		return "progress"
	}
	if containsFold(reason, "completed") || containsFold(reason, "succeed") || containsFold(reason, "healthy") || containsFold(reason, "synced") || containsFold(reason, "updated") {
		return "success"
	}
	return "info"
}

// ShortRevision keeps a long hex sha readable and leaves tags whole.
func ShortRevision(rev string) string {
	if len(rev) >= 12 && hexString(rev) {
		return rev[:7]
	}
	return rev
}

func shown(status string) string {
	if status == "" {
		return "Unknown"
	}
	return status
}

func containsFold(s, part string) bool {
	return strings.Contains(strings.ToLower(s), part)
}

func hexString(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.ASCII_Hex_Digit, r) {
			continue
		}
		return false
	}
	return s != ""
}
