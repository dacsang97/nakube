package desktop

import (
	"testing"

	"github.com/dacsang97/nakube/internal/agent"
	"github.com/dacsang97/nakube/internal/cluster"
)

func TestExecTitleUpdatesAndClearsAgentIdentity(t *testing.T) {
	a := New(cluster.NewSession())
	a.active = "tab"
	a.mode = "exec"
	a.execGen = 2
	a.bodies = map[string]*tabBody{"tab": {mode: "exec", execGen: 2}}

	a.noteExecTitle("tab", 2, "Claude Code")
	want := agent.Identity{ID: "claude", Name: "Claude Code"}
	if a.execAgent != want || a.bodies["tab"].execAgent != want {
		t.Fatalf("identity not projected: active=%+v tab=%+v", a.execAgent, a.bodies["tab"].execAgent)
	}

	a.noteExecTitle("tab", 2, "fix codex bug")
	if a.execAgent != (agent.Identity{}) || a.bodies["tab"].execAgent != (agent.Identity{}) {
		t.Fatalf("stale identity was not cleared: active=%+v tab=%+v", a.execAgent, a.bodies["tab"].execAgent)
	}

	a.noteExecTitle("tab", 1, "Codex")
	a.noteExecErr("tab", 1, "stale connection failed")
	if a.execAgent != (agent.Identity{}) || a.execErr != "" || a.bodies["tab"].execErr != "" {
		t.Fatalf("stale generation changed current state: agent=%+v err=%q", a.execAgent, a.execErr)
	}
}
