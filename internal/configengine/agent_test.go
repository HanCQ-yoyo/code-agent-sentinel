package configengine

import (
	"path/filepath"
	"testing"
)

func TestDefaultAgents(t *testing.T) {
	agents := DefaultAgents("/home/alice")
	if len(agents) != 1 {
		t.Fatalf("本轮只注册 Claude Code,期望 1 个 agent,实际 %d", len(agents))
	}
	a := agents[0]
	if a.ID != "claude-code" {
		t.Errorf("ID = %q, 期望 claude-code", a.ID)
	}
	if a.Name != "Claude Code" {
		t.Errorf("Name = %q, 期望 Claude Code", a.Name)
	}
	if a.RootDir != filepath.Join("/home/alice", ".claude") {
		t.Errorf("RootDir = %q, 期望 ~/.claude", a.RootDir)
	}
	if a.ClaudeJSON != filepath.Join("/home/alice", ".claude.json") {
		t.Errorf("ClaudeJSON = %q, 期望 ~/.claude.json", a.ClaudeJSON)
	}
	if a.HomeDir != "/home/alice" {
		t.Errorf("HomeDir = %q, 期望 /home/alice", a.HomeDir)
	}
}

func TestNewEngineFromAgent(t *testing.T) {
	agents := DefaultAgents("/home/alice")
	eng := NewEngineFromAgent(agents[0])
	if eng.HomeDir != "/home/alice" {
		t.Errorf("HomeDir = %q, 期望 /home/alice", eng.HomeDir)
	}
	if eng.ClaudeJSON != agents[0].ClaudeJSON {
		t.Errorf("ClaudeJSON = %q, 期望 %q", eng.ClaudeJSON, agents[0].ClaudeJSON)
	}
}
