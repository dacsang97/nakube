// Package agent identifies coding-agent CLIs from exact terminal title reports.
package agent

import "strings"

// Identity is the stable coding-agent identity exposed to desktop views.
// Its zero value means the terminal title did not identify a known agent.
type Identity struct {
	ID   string
	Name string
}

var titleIdentities = map[string]Identity{
	"aider":          {ID: "aider", Name: "Aider"},
	"amp":            {ID: "amp", Name: "Amp"},
	"claude":         {ID: "claude", Name: "Claude Code"},
	"claude code":    {ID: "claude", Name: "Claude Code"},
	"claude-code":    {ID: "claude", Name: "Claude Code"},
	"codex":          {ID: "codex", Name: "Codex"},
	"copilot":        {ID: "copilot", Name: "GitHub Copilot"},
	"cursor":         {ID: "cursor", Name: "Cursor Agent"},
	"cursor agent":   {ID: "cursor", Name: "Cursor Agent"},
	"cursor-agent":   {ID: "cursor", Name: "Cursor Agent"},
	"gemini":         {ID: "gemini", Name: "Gemini"},
	"gemini cli":     {ID: "gemini", Name: "Gemini"},
	"github copilot": {ID: "copilot", Name: "GitHub Copilot"},
	"goose":          {ID: "goose", Name: "Goose"},
	"kiro":           {ID: "kiro", Name: "Kiro"},
	"kiro cli":       {ID: "kiro", Name: "Kiro"},
	"kiro-cli":       {ID: "kiro", Name: "Kiro"},
	"omp":            {ID: "omp", Name: "OMP"},
	"opencode":       {ID: "opencode", Name: "OpenCode"},
	"pi":             {ID: "pi", Name: "Pi"},
	"qwen code":      {ID: "qwen", Name: "Qwen Code"},
	"qwen-code":      {ID: "qwen", Name: "Qwen Code"},
}

// DetectTitle recognizes only a complete, exact title after whitespace and
// case normalization. Task titles that merely mention an agent do not match.
func DetectTitle(title string) Identity {
	return titleIdentities[strings.ToLower(strings.Join(strings.Fields(title), " "))]
}
