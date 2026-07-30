package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/history"
	"code-agent-sentinel/internal/security"
)

// findingsStateAndMetaSetup 构造单 agent fixture 用于验证 /api/findings 读路径
// apply state/meta。返回 (server, fingerprint, startedAt),供单 agent 与聚合路径
// 两个测试共用。fixture 细节见 TestGetFindingsAppliesStateAndMeta 注释。
func findingsStateAndMetaSetup(t *testing.T) (s *Server, fp string, started time.Time) {
	t.Helper()
	dir := t.TempDir()
	s = newTestServer(t, dir)

	fp = "fp-test-123"
	started = time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	rec := history.ScanRecord{
		ID:        "2026-07-30-12-00-00-aaaaaaaa",
		AgentID:   "claude-code",
		StartedAt: started,
		Scope:     "global",
		Findings: []security.Finding{
			{
				ID:          "f1",
				Severity:    security.SeverityHigh,
				AssetID:     "asset-1",
				Fingerprint: fp,
			},
		},
		Inventory: &configengine.Inventory{
			Assets: []configengine.Asset{
				{ID: "asset-1", SourcePath: "/home/u/.claude/settings.json"},
			},
		},
	}
	if err := s.History.Save(rec); err != nil {
		t.Fatalf("save rec: %v", err)
	}

	// 写 finding_states.yaml:{ items: [{ fingerprint: fp-test-123, status: resolved }] }
	statesPath := filepath.Join(dir, ".claude-sentinel", "finding_states.yaml")
	yaml := "items:\n  - fingerprint: " + fp + "\n    status: resolved\n"
	if err := os.MkdirAll(filepath.Dir(statesPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(statesPath, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write finding_states.yaml: %v", err)
	}
	return s, fp, started
}

// assertFindingStateAndMeta 断言单条 finding 的四个 apply-state/meta 字段。
// 供单 agent 与聚合路径测试共享,确保两路径行为一致。
func assertFindingStateAndMeta(t *testing.T, findings []security.Finding, started time.Time) {
	t.Helper()
	if len(findings) != 1 {
		t.Fatalf("应返回 1 条 finding: got %d", len(findings))
	}
	f := findings[0]
	if !f.Suppressed {
		t.Errorf("Suppressed = false, 期望 true(命中 resolved)")
	}
	if f.Status != "resolved" {
		t.Errorf("Status = %q, 期望 resolved", f.Status)
	}
	if f.StartedAt.IsZero() {
		t.Errorf("StartedAt 为零值,期望来自 ScanRecord.StartedAt")
	} else if !f.StartedAt.Equal(started) {
		t.Errorf("StartedAt = %v, 期望 %v", f.StartedAt, started)
	}
	if f.SourcePath == "" {
		t.Errorf("SourcePath 为空,期望 /home/u/.claude/settings.json")
	} else if f.SourcePath != "/home/u/.claude/settings.json" {
		t.Errorf("SourcePath = %q, 期望 /home/u/.claude/settings.json", f.SourcePath)
	}
}

// TestGetFindingsAppliesStateAndMeta 验证 /api/findings 单 agent 读路径
// (?agent=claude-code,经 getFindings)在过滤后调 security.ApplyFindingStateBatch:
// 合并 finding_states.yaml 的处置状态(命中 resolved → Suppressed=true + Status=resolved),
// 并附 StartedAt(来自 ScanRecord)与 SourcePath(来自 ScanRecord.Inventory 快照)。
//
// fixture 模式参考 TestGetFindingsAgentAll(aggregate_test.go):直接向 s.History
// 注入一条 ScanRecord(已知 fingerprint + Inventory + StartedAt),绕开扫描触发,
// 使 fingerprint 可控;finding_states.yaml 手写到 home/.claude-sentinel/ 下。
func TestGetFindingsAppliesStateAndMeta(t *testing.T) {
	s, _, started := findingsStateAndMetaSetup(t)

	// GET /api/findings?agent=claude-code(单 agent 路径,经 getFindings)
	r := s.Router()
	req := httptest.NewRequest("GET", "/api/findings?agent=claude-code", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var findings []security.Finding
	if err := json.NewDecoder(w.Body).Decode(&findings); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertFindingStateAndMeta(t, findings, started)
}

// TestGetFindingsAggregatedAppliesStateAndMeta 验证 /api/findings 聚合读路径
// (?agent=all,经 getFindingsAggregated)同样调 ApplyFindingStateBatch。单 agent
// fixture 上 ?agent=all 仍走聚合路径(shouldAggregate 显式判断 agent==all → true,
// 见 aggregate.go:44-45),故无需第二个 agent。补 Task 13 review 缺口:聚合路径
// 原无测试覆盖,删 ApplyFindingStateBatch 调用不会被现有测试捕获。
func TestGetFindingsAggregatedAppliesStateAndMeta(t *testing.T) {
	s, _, started := findingsStateAndMetaSetup(t)

	// GET /api/findings?agent=all(聚合路径,经 getFindingsAggregated)
	r := s.Router()
	req := httptest.NewRequest("GET", "/api/findings?agent=all", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var findings []security.Finding
	if err := json.NewDecoder(w.Body).Decode(&findings); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertFindingStateAndMeta(t, findings, started)
}
