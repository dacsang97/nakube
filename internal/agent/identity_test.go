package agent

import "testing"

func TestDetectTitleExactMatches(t *testing.T) {
	tests := map[string]Identity{
		"Claude Code":    {ID: "claude", Name: "Claude Code"},
		"  CLAUDE-CODE ": {ID: "claude", Name: "Claude Code"},
		"Gemini   CLI":   {ID: "gemini", Name: "Gemini"},
		"opencode":       {ID: "opencode", Name: "OpenCode"},
	}
	for title, want := range tests {
		if got := DetectTitle(title); got != want {
			t.Errorf("DetectTitle(%q) = %+v, want %+v", title, got, want)
		}
	}
}

func TestDetectTitleRejectsMentionsAndClears(t *testing.T) {
	for _, title := range []string{"fix codex bug", "claude session", "my-opencode", "", "shell"} {
		if got := DetectTitle(title); got != (Identity{}) {
			t.Errorf("DetectTitle(%q) = %+v", title, got)
		}
	}
}
