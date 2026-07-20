package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/history"
	"code-agent-sentinel/internal/security"
)

func TestRunnerRunScanWritesHistory(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claude, "settings.json"), []byte(`{"permissions":{"allow":["Bash(*)"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	agents := configengine.DefaultAgents(dir, "")
	r := security.NewRegistry()
	r.Register(security.NewRulesDetector(dir, nil))
	orch := &security.Orchestrator{Registry: r}
	hist := history.NewStore(filepath.Join(dir, "history"))
	runner := NewRunner(agents, orch, hist)

	res, err := runner.RunScan(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("RunScan: %v", err)
	}
	if len(res.Findings) == 0 {
		t.Error("应检出通配 Bash")
	}
	if res.HealthScore == nil {
		t.Error("应有健康分")
	}
	// 历史应落盘 1 条(List 返回 ScanSummary 列表,按 StartedAt 倒序)
	recs, err := hist.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 1 {
		t.Errorf("历史应 1 条,got %d", len(recs))
	}
}

func TestRunnerNilHistoryNoPanic(t *testing.T) {
	dir := t.TempDir()
	agents := configengine.DefaultAgents(dir, "")
	r := security.NewRegistry()
	r.Register(security.NewRulesDetector(dir, nil))
	orch := &security.Orchestrator{Registry: r}
	runner := NewRunner(agents, orch, nil) // History nil
	res, err := runner.RunScan(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("RunScan: %v", err)
	}
	if res == nil {
		t.Error("res 不应 nil")
	}
}

func TestRunScanSelectsAgentByID(t *testing.T) {
	// 构造两个 agent 指向不同 claudeDir,验证 RunScan(agentID) 扫对应目录。
	home := t.TempDir()
	dirA := filepath.Join(home, "agentA")
	dirB := filepath.Join(home, "agentB")
	os.MkdirAll(filepath.Join(dirA, ".claude"), 0o755)
	os.MkdirAll(filepath.Join(dirB, ".claude"), 0o755)
	// 各放一个不同 settings.json 以区分
	os.WriteFile(filepath.Join(dirA, ".claude", "settings.json"), []byte(`{"model":"A"}`), 0o644)
	os.WriteFile(filepath.Join(dirB, ".claude", "settings.json"), []byte(`{"model":"B"}`), 0o644)

	agents := []configengine.Agent{
		{ID: "a", Name: "A", RootDir: filepath.Join(dirA, ".claude"), ClaudeJSON: "", HomeDir: home},
		{ID: "b", Name: "B", RootDir: filepath.Join(dirB, ".claude"), ClaudeJSON: "", HomeDir: home},
	}
	r := NewRunner(agents, &security.Orchestrator{}, nil)
	eng := r.EngineFor("b")
	if eng == nil || eng.ClaudeDir != filepath.Join(dirB, ".claude") {
		t.Fatalf("EngineFor(b) 应返回 B 的 Engine: %+v", eng)
	}
}

func TestRunScanFallsBackToFirstAgentWhenIDEmpty(t *testing.T) {
	home := t.TempDir()
	agents := []configengine.Agent{
		{ID: "first", RootDir: filepath.Join(home, ".claude1"), HomeDir: home},
		{ID: "second", RootDir: filepath.Join(home, ".claude2"), HomeDir: home},
	}
	r := NewRunner(agents, &security.Orchestrator{}, nil)
	eng := r.EngineFor("")
	if eng == nil || eng.ClaudeDir != filepath.Join(home, ".claude1") {
		t.Fatalf("空 agentID 应回退首 agent: %+v", eng)
	}
}

func TestEngineForCachesByAgentID(t *testing.T) {
	home := t.TempDir()
	agents := []configengine.Agent{{ID: "x", RootDir: filepath.Join(home, ".claude"), HomeDir: home}}
	r := NewRunner(agents, &security.Orchestrator{}, nil)
	e1 := r.EngineFor("x")
	e2 := r.EngineFor("x")
	if e1 != e2 {
		t.Fatal("同 agentID 应返回缓存的同一 Engine 实例")
	}
}
