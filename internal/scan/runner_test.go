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
	eng := configengine.NewEngine(dir, "")
	r := security.NewRegistry()
	r.Register(security.NewRulesDetector(dir, nil))
	orch := &security.Orchestrator{Registry: r}
	hist := history.NewStore(filepath.Join(dir, "history"))
	runner := NewRunner(eng, orch, hist)

	res, err := runner.RunScan(context.Background(), nil)
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
	eng := configengine.NewEngine(dir, "")
	r := security.NewRegistry()
	r.Register(security.NewRulesDetector(dir, nil))
	orch := &security.Orchestrator{Registry: r}
	runner := NewRunner(eng, orch, nil) // History nil
	res, err := runner.RunScan(context.Background(), nil)
	if err != nil {
		t.Fatalf("RunScan: %v", err)
	}
	if res == nil {
		t.Error("res 不应 nil")
	}
}
