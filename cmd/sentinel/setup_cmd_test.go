package main

import (
	"os"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/config"
)

// TestDetectAgentsReturnsExisting 验证 home 下有 .claude 时,detectAgents 返回 claude-code。
func TestDetectAgentsReturnsExisting(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := detectAgents(home)
	if len(got) != 1 || got[0].ID != "claude-code" {
		t.Errorf("应检测到 claude-code: %+v", got)
	}
}

// TestDetectAgentsEmptyWhenNone 验证 home 下无 .claude 时,detectAgents 返回空。
func TestDetectAgentsEmptyWhenNone(t *testing.T) {
	home := t.TempDir()
	got := detectAgents(home)
	if len(got) != 0 {
		t.Errorf("无 .claude 应返回空: %+v", got)
	}
}

// TestRunSetupRejectsNonTTY 验证 stdin 非 TTY 时 runSetup 报错(管道模拟)。
// 非交互式 CI 无法跑 happy path,只测拒绝分支。
func TestRunSetupRejectsNonTTY(t *testing.T) {
	r, w, _ := os.Pipe()
	defer w.Close()
	err := runSetup("", "", false, r, os.Stdout)
	if err == nil {
		t.Fatal("非 TTY 应报错")
	}
}

// TestDetectAgentsFindsCodex 验证 home 下有 ~/.codex/config.toml 时,detectAgents 返回 codex。
// Task 1 已让 KnownAgents() 含 codex spec(Detect 检查 ~/.codex/config.toml),此处只验证端到端。
func TestDetectAgentsFindsCodex(t *testing.T) {
	home := t.TempDir()
	// 无任何 agent → 空
	if got := detectAgents(home); len(got) != 0 {
		t.Fatalf("空 home 应探测到 0 agent, got %d", len(got))
	}
	// 造 ~/.codex/config.toml
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("model = \"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := detectAgents(home)
	var ids []string
	for _, s := range got {
		ids = append(ids, s.ID)
	}
	found := false
	for _, id := range ids {
		if id == "codex" {
			found = true
		}
	}
	if !found {
		t.Fatalf("应探测到 codex, got %v", ids)
	}
}

// TestMergeSetupSelectionIntoConfig 验证 mergeAgents 把选择写入 cfg.Agents 且保留其他字段。
func TestMergeSetupSelectionIntoConfig(t *testing.T) {
	selection := []config.AgentCfg{
		{ID: "claude-code", Enabled: true, RootDir: "/x/.claude", ClaudeJSON: "/x/.claude.json"},
	}
	cfg := &config.Config{Language: "en"} // 其他字段应保留
	mergeAgents(cfg, selection)
	if len(cfg.Agents) != 1 || cfg.Agents[0].RootDir != "/x/.claude" {
		t.Errorf("merge 错: %+v", cfg.Agents)
	}
	if cfg.Language != "en" {
		t.Error("merge 不应破坏其他字段")
	}
}
