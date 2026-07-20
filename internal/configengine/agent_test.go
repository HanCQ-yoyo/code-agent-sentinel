package configengine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultAgents(t *testing.T) {
	agents := DefaultAgents("/home/alice", "")
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
	agents := DefaultAgents("/home/alice", "")
	eng := NewEngineFromAgent(agents[0])
	if eng.HomeDir != "/home/alice" {
		t.Errorf("HomeDir = %q, 期望 /home/alice", eng.HomeDir)
	}
	if eng.ClaudeJSON != agents[0].ClaudeJSON {
		t.Errorf("ClaudeJSON = %q, 期望 %q", eng.ClaudeJSON, agents[0].ClaudeJSON)
	}
}

func TestKnownAgentsContainsClaudeCode(t *testing.T) {
	specs := KnownAgents()
	if len(specs) == 0 {
		t.Fatal("KnownAgents 不应为空")
	}
	var cc *AgentSpec
	for i := range specs {
		if specs[i].ID == "claude-code" {
			cc = &specs[i]
		}
	}
	if cc == nil {
		t.Fatal("KnownAgents 应含 claude-code")
	}
	home := t.TempDir()
	if cc.DefaultRootDir(home) != filepath.Join(home, ".claude") {
		t.Errorf("claude-code DefaultRootDir 应 home/.claude")
	}
	if !cc.HasClaudeJSON {
		t.Error("claude-code HasClaudeJSON 应 true")
	}
	if cc.DefaultClaudeJSON(home) != filepath.Join(home, ".claude.json") {
		t.Errorf("DefaultClaudeJSON 应 home/.claude.json")
	}
	// Detect:home 下无 .claude 时 false
	if cc.Detect(home) {
		t.Error("无 .claude 时 Detect 应 false")
	}
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	if !cc.Detect(home) {
		t.Error("有 .claude 时 Detect 应 true")
	}
}

func TestAgentsFromSpecsMapsItems(t *testing.T) {
	home := t.TempDir()
	items := []AgentItem{
		{ID: "claude-code", Enabled: true, RootDir: "/x/.claude", ClaudeJSON: "/x/.claude.json"},
	}
	agents := AgentsFromSpecs(home, items)
	if len(agents) != 1 || agents[0].ID != "claude-code" || agents[0].RootDir != "/x/.claude" || agents[0].HomeDir != home {
		t.Fatalf("映射错: %+v", agents)
	}
}
