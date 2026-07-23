// internal/history/store_test.go
package history

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security"
)

func TestSaveGetLatest(t *testing.T) {
	s := NewStore(t.TempDir())
	rec := ScanRecord{
		ID:        "2026-07-06-14-30-05-a1b2c3d4",
		StartedAt: time.Date(2026, 7, 6, 14, 30, 5, 0, time.UTC),
		Findings: []security.Finding{
			{DetectorID: "baseline", RuleID: "r1", Severity: security.SeverityHigh, AssetID: "a1"},
		},
		HealthScore: &security.HealthScore{Score: 80, Band: "良"},
		Inventory:   &configengine.Inventory{Assets: []configengine.Asset{{ID: "a1", Type: "settings", Name: "n"}}},
	}
	if err := s.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Get(rec.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != rec.ID || len(got.Findings) != 1 || got.HealthScore.Score != 80 {
		t.Fatalf("往返不一致: %+v", got)
	}
	latest, err := s.Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if latest.ID != rec.ID {
		t.Fatalf("Latest 应为刚存的记录,got %s", latest.ID)
	}
}

func TestListOrderAndEmpty(t *testing.T) {
	s := NewStore(t.TempDir())
	got, err := s.List()
	if err != nil {
		t.Fatalf("空目录 List: %v", err)
	}
	if got != nil {
		t.Fatalf("空目录应返回 nil,got %+v", got)
	}
	// 存 3 条,时间递增
	for i, ts := range []time.Time{
		time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 6, 11, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC),
	} {
		rec := ScanRecord{ID: fmt.Sprintf("rec-%d", i), StartedAt: ts}
		if err := s.Save(rec); err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
	}
	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("应有 3 条,got %d", len(list))
	}
	if list[0].ID != "rec-2" || list[2].ID != "rec-0" {
		t.Fatalf("倒序错误: %+v", list)
	}
}

func TestDeleteAndNotFound(t *testing.T) {
	s := NewStore(t.TempDir())
	rec := ScanRecord{ID: "del-me", StartedAt: time.Now().UTC()}
	if err := s.Save(rec); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(rec.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(rec.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删除后应 ErrNotFound,got %v", err)
	}
	if err := s.Delete("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删不存在应 ErrNotFound,got %v", err)
	}
	if _, err := s.Latest(); err != nil {
		t.Fatalf("空目录 Latest 应 (nil,nil),got %v", err)
	}
}

func TestSameSecondNoConflict(t *testing.T) {
	s := NewStore(t.TempDir())
	ts := time.Date(2026, 7, 6, 14, 30, 5, 0, time.UTC)
	for i := 0; i < 3; i++ {
		rec := ScanRecord{ID: fmt.Sprintf("2026-07-06-14-30-05-%08d", i), StartedAt: ts}
		if err := s.Save(rec); err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("同秒 3 条应全部保留,got %d", len(list))
	}
}

func TestListSummaryFields(t *testing.T) {
	s := NewStore(t.TempDir())
	rec := ScanRecord{
		ID:        "2026-07-06-14-30-05-a1b2c3d4",
		StartedAt: time.Date(2026, 7, 6, 14, 30, 5, 0, time.UTC),
		Findings: []security.Finding{
			{DetectorID: "baseline", RuleID: "r1", Severity: security.SeverityHigh, AssetID: "a1"},
			{DetectorID: "secrets", RuleID: "r2", Severity: security.SeverityLow, AssetID: "a2"},
		},
		Detectors: []security.DetectorStatus{
			{ID: "baseline", Available: true},
			{ID: "secrets", Available: false, Reason: "gitleaks missing"},
		},
		HealthScore: &security.HealthScore{Score: 75, Band: "良"},
	}
	if err := s.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("应有 1 条,got %d", len(list))
	}
	sum := list[0]
	if sum.FindingCount != 2 {
		t.Errorf("FindingCount = %d, want 2", sum.FindingCount)
	}
	if sum.DetectorAvail != 1 {
		t.Errorf("DetectorAvail = %d, want 1", sum.DetectorAvail)
	}
	if sum.DetectorTotal != 2 {
		t.Errorf("DetectorTotal = %d, want 2", sum.DetectorTotal)
	}
	if sum.HealthScore != 75 {
		t.Errorf("HealthScore = %d, want 75", sum.HealthScore)
	}
	if sum.Band != "良" {
		t.Errorf("Band = %q, want \"良\"", sum.Band)
	}
}

func TestScanRecordHasAgentID(t *testing.T) {
	rec := ScanRecord{ID: "x", AgentID: "claude-code"}
	if rec.AgentID != "claude-code" {
		t.Errorf("AgentID 字段应可读写: %q", rec.AgentID)
	}
}

func TestScanSummaryHasAgentID(t *testing.T) {
	s := ScanSummary{ID: "x", AgentID: "claude-code"}
	if s.AgentID != "claude-code" {
		t.Errorf("AgentID 字段应可读写: %q", s.AgentID)
	}
}

// TestHistoryLegacyDetectorID 验证 P3 重构(基线+注入检测器合并为 rules 检测器)
// 之前写入的旧记录仍可被读取,不崩溃,finding 字段完整保留,且 rule_id 前缀
// 分组不变——前端 RulesTable(Task 17)按 rule_id 前缀分组,旧记录的
// "baseline." / "injection." 前缀仍需可识别,以归入"基线"/"注入"分组。
// 旧 detector_id="baseline" / "content.injection" 在 DetectorStatus 里仍保留
// 中文名映射(Task 17),故此处不校验 detector 名,只校验 id 往返 + rule_id 前缀。
func TestHistoryLegacyDetectorID(t *testing.T) {
	s := NewStore(t.TempDir())
	rec := ScanRecord{
		ID:        "2026-07-06-14-30-05-legacy01",
		StartedAt: time.Date(2026, 7, 6, 14, 30, 5, 0, time.UTC),
		// 旧记录:detector_id 用重构前的 baseline / content.injection,
		// rule_id 用旧前缀(baseline.* / injection.*)。新规则包(Task 12-14)
		// 虽然改用 rules.* 命名,但历史记录不受影响。
		Findings: []security.Finding{
			{DetectorID: "baseline", RuleID: "baseline.wildcard-bash", Severity: security.SeverityHigh, AssetID: "a1"},
			{DetectorID: "content.injection", RuleID: "injection.hidden-instruction", Severity: security.SeverityCritical, AssetID: "a2"},
		},
		HealthScore: &security.HealthScore{Score: 60, Band: "中"},
	}
	if err := s.Save(rec); err != nil {
		t.Fatalf("Save 旧记录: %v", err)
	}
	got, err := s.Get(rec.ID)
	if err != nil {
		t.Fatalf("Get 旧记录不应报错(legacy-compat): %v", err)
	}
	if len(got.Findings) != 2 {
		t.Fatalf("finding 数量应保留为 2,got %d", len(got.Findings))
	}
	// 逐条校验字段往返完整,且 rule_id 前缀可识别(前端分组不变式)。
	want := rec.Findings
	for i, f := range got.Findings {
		w := want[i]
		if f.DetectorID != w.DetectorID {
			t.Errorf("finding[%d].DetectorID = %q, want %q", i, f.DetectorID, w.DetectorID)
		}
		if f.RuleID != w.RuleID {
			t.Errorf("finding[%d].RuleID = %q, want %q", i, f.RuleID, w.RuleID)
		}
		if f.Severity != w.Severity {
			t.Errorf("finding[%d].Severity = %q, want %q", i, f.Severity, w.Severity)
		}
		if f.AssetID != w.AssetID {
			t.Errorf("finding[%d].AssetID = %q, want %q", i, f.AssetID, w.AssetID)
		}
	}
	// 前端分组不变式:旧 rule_id 仍带可识别前缀(baseline. / injection.),
	// RulesTable 按 "." 之前的前缀分组 → 旧记录归入"基线"/"注入"组,不丢失。
	if !strings.HasPrefix(got.Findings[0].RuleID, "baseline.") {
		t.Errorf("legacy rule_id %q 缺少 \"baseline.\" 前缀,前端分组会断裂", got.Findings[0].RuleID)
	}
	if !strings.HasPrefix(got.Findings[1].RuleID, "injection.") {
		t.Errorf("legacy rule_id %q 缺少 \"injection.\" 前缀,前端分组会断裂", got.Findings[1].RuleID)
	}
	// 新增字段(Suppressed/Suppression/Reason/Fingerprint)在旧记录 JSON 里不存在,
	// Go json.Unmarshal 对缺失字段赋零值,不应报错。这里隐式验证:got 反序列化成功
	// 且这些字段为零值(无抑制标记)。
	if got.Findings[0].Suppressed {
		t.Errorf("legacy finding 不应有 Suppressed=true")
	}
}

func TestLatestForAgent(t *testing.T) {
	s := NewStore(t.TempDir())
	// 两条不同 agent 的记录(同 StartedAt 不易构造,用不同时间)
	recA := ScanRecord{ID: "a-1", AgentID: "a", StartedAt: time.Now().Add(-2 * time.Hour)}
	recB := ScanRecord{ID: "b-1", AgentID: "b", StartedAt: time.Now().Add(-1 * time.Hour)}
	recA2 := ScanRecord{ID: "a-2", AgentID: "a", StartedAt: time.Now()}
	s.Save(recA)
	s.Save(recB)
	s.Save(recA2)
	// LatestForAgent("a") 应返回 a-2(最新)
	got, err := s.LatestForAgent("a")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != "a-2" {
		t.Fatalf("LatestForAgent(a) 应返回 a-2: got %+v", got)
	}
	// LatestForAgent("b") 应返回 b-1
	gotB, _ := s.LatestForAgent("b")
	if gotB == nil || gotB.ID != "b-1" {
		t.Fatalf("LatestForAgent(b) 应返回 b-1: got %+v", gotB)
	}
	// 空 agentID 退化为 Latest()(全局最新 = a-2)
	gotAll, _ := s.LatestForAgent("")
	if gotAll == nil || gotAll.ID != "a-2" {
		t.Fatalf("LatestForAgent(空) 应退化为全局最新: got %+v", gotAll)
	}
}

func TestBatchIDRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	rec := ScanRecord{
		ID:        "test-batch-1",
		AgentID:   "claude-code",
		BatchID:   "batch-abc",
		StartedAt: time.Now(),
	}
	if err := s.Save(rec); err != nil {
		t.Fatalf("save: %v", err)
	}
	sums, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sums) != 1 || sums[0].BatchID != "batch-abc" {
		t.Errorf("BatchID roundtrip failed: got %q", sums[0].BatchID)
	}
}

func TestBatchIDEmptyBackwardCompat(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	// 旧 record 无 BatchID → 空串
	rec := ScanRecord{ID: "old-record", AgentID: "claude-code", StartedAt: time.Now()}
	if err := s.Save(rec); err != nil {
		t.Fatalf("save: %v", err)
	}
	sums, _ := s.List()
	if sums[0].BatchID != "" {
		t.Errorf("旧 record BatchID 应为空串: got %q", sums[0].BatchID)
	}
}

func TestLatestForAgentPrefersGlobalScope(t *testing.T) {
	s := NewStore(t.TempDir())
	// a 的 project scope 扫描(较新)+ global scope 扫描(较旧)
	s.Save(ScanRecord{ID: "a-global", AgentID: "a", Scope: "global", StartedAt: time.Now().Add(-1 * time.Hour)})
	s.Save(ScanRecord{ID: "a-proj", AgentID: "a", Scope: "project", ScopePath: "/p", StartedAt: time.Now()})
	// LatestForAgent("a") 应返回 a-global(虽较旧但 scope=global)
	got, _ := s.LatestForAgent("a")
	if got == nil || got.ID != "a-global" {
		t.Fatalf("应取 scope=global 的最新: got %+v", got)
	}
}
