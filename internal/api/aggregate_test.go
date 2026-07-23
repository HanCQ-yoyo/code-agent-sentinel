package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"code-agent-sentinel/internal/history"
	"code-agent-sentinel/internal/security"
)

// reqDashboard 发 GET 请求并返回 recorder(用于 dashboard/findings/health 聚合测试)。
func reqDashboard(t *testing.T, s *Server, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := s.Router()
	req := httptest.NewRequest(method, path, nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestAggregateDashboardSingleAgentAll 验证 ?agent=all 即使在单 agent fixture 上也走聚合路径
// (resolveAgentIDs 返回 ["claude-code"],len==1,但 ?agent=all 显式请求聚合 → 必须聚合)。
// 聚合响应应含 agent_scans 数组 + is_aggregate=true,而非单 agent 的 agent/agent_name 字段。
func TestAggregateDashboardSingleAgentAll(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	w := reqDashboard(t, s, "GET", "/api/dashboard?agent=all")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["is_aggregate"] != true {
		t.Errorf("?agent=all 应 is_aggregate=true: got %v", body["is_aggregate"])
	}
	scans, ok := body["agent_scans"].([]any)
	if !ok {
		t.Fatalf("聚合模式应返回 agent_scans 数组: got %T", body["agent_scans"])
	}
	if len(scans) != 1 {
		t.Errorf("单 agent fixture 应返回 1 条 agent_scans: got %d", len(scans))
	}
	// 不应有单 agent 模式的 agent/agent_name 顶层字段
	if _, has := body["agent"]; has {
		t.Error("聚合模式不应返回顶层 agent 字段")
	}
}

// TestGetDashboardAgentAll 验证双 agent fixture 上 ?agent=all 返回 2 条 agent_scans,
// 每条含 agent_id/agent_name,asset_counts 汇总两 agent 的资产计数。
func TestGetDashboardAgentAll(t *testing.T) {
	dir := t.TempDir()
	s := newTwoAgentTestServer(t, dir)
	w := reqDashboard(t, s, "GET", "/api/dashboard?agent=all")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["is_aggregate"] != true {
		t.Errorf("应 is_aggregate=true: got %v", body["is_aggregate"])
	}
	scans, ok := body["agent_scans"].([]any)
	if !ok {
		t.Fatalf("应返回 agent_scans 数组: got %T", body["agent_scans"])
	}
	if len(scans) != 2 {
		t.Errorf("双 agent 应返回 2 条 agent_scans: got %d", len(scans))
	}
	// 每条应含 agent_id 与 agent_name
	ids := map[string]bool{}
	for _, sc := range scans {
		m, _ := sc.(map[string]any)
		id, _ := m["agent_id"].(string)
		ids[id] = true
		if name, _ := m["agent_name"].(string); name == "" {
			t.Error("agent_scans 项缺 agent_name")
		}
	}
	if !ids["a"] || !ids["b"] {
		t.Errorf("agent_scans 应含 a 与 b: got %v", ids)
	}
	// asset_counts 应存在(汇总两 agent 的资产)
	if body["asset_counts"] == nil {
		t.Error("聚合模式应返回汇总 asset_counts")
	}
}

// TestGetDashboardAgentCommaList 验证 ?agent=a,b(逗号列表)走聚合路径,
// 返回 2 条 agent_scans。与 ?agent=all 同路径,但显式列出 agent ID。
func TestGetDashboardAgentCommaList(t *testing.T) {
	dir := t.TempDir()
	s := newTwoAgentTestServer(t, dir)
	w := reqDashboard(t, s, "GET", "/api/dashboard?agent=a,b")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["is_aggregate"] != true {
		t.Errorf("?agent=a,b 应 is_aggregate=true: got %v", body["is_aggregate"])
	}
	scans, _ := body["agent_scans"].([]any)
	if len(scans) != 2 {
		t.Errorf("?agent=a,b 应返回 2 条 agent_scans: got %d", len(scans))
	}
}

// TestGetDashboardAggregateUnknownAgent 验证聚合模式对未知 agent 仍报 400。
// ?agent=a,zzz 中 zzz 不存在 → 400 unknown_agent(不静默跳过)。
func TestGetDashboardAggregateUnknownAgent(t *testing.T) {
	dir := t.TempDir()
	s := newTwoAgentTestServer(t, dir)
	w := reqDashboard(t, s, "GET", "/api/dashboard?agent=a,zzz")
	if w.Code != http.StatusBadRequest {
		t.Errorf("未知 agent 应 400: got %d %s", w.Code, w.Body.String())
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	errObj, _ := body["error"].(map[string]any)
	if code, _ := errObj["code"].(string); code != "unknown_agent" {
		t.Errorf("应 unknown_agent: got %v", errObj)
	}
}

// TestGetFindingsAgentAll 验证 ?agent=all 拼接各 agent 最近 global 扫描的 findings。
// 手动向 History 注入两条 ScanRecord(不同 AgentID,各带一条 finding),
// GET /api/findings?agent=all 应返回 2 条 finding,每条带对应的 AgentID。
func TestGetFindingsAgentAll(t *testing.T) {
	dir := t.TempDir()
	s := newTwoAgentTestServer(t, dir)
	// 注入两条历史记录(各 agent 一条 global scope),各带 1 条不同 AgentID 的 finding
	now := time.Now()
	recA := history.ScanRecord{
		ID:        "2026-01-01-00-00-00-aaaaaaaa",
		AgentID:   "a",
		StartedAt: now.Add(-1 * time.Minute),
		Scope:     "global",
		Findings: []security.Finding{
			{ID: "f-a", AgentID: "a", Severity: security.SeverityHigh, AssetID: "asset-a"},
		},
		HealthScore: &security.HealthScore{Score: 80, Band: "fair"},
	}
	recB := history.ScanRecord{
		ID:        "2026-01-01-00-00-00-bbbbbbbb",
		AgentID:   "b",
		StartedAt: now,
		Scope:     "global",
		Findings: []security.Finding{
			{ID: "f-b", AgentID: "b", Severity: security.SeverityLow, AssetID: "asset-b"},
		},
		HealthScore: &security.HealthScore{Score: 90, Band: "good"},
	}
	if err := s.History.Save(recA); err != nil {
		t.Fatalf("save recA: %v", err)
	}
	if err := s.History.Save(recB); err != nil {
		t.Fatalf("save recB: %v", err)
	}

	w := reqDashboard(t, s, "GET", "/api/findings?agent=all")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	var findings []security.Finding
	if err := json.NewDecoder(w.Body).Decode(&findings); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("应返回 2 条 finding(各 agent 1 条): got %d", len(findings))
	}
	byAgent := map[string]string{}
	for _, f := range findings {
		byAgent[f.AgentID] = f.ID
	}
	if byAgent["a"] != "f-a" {
		t.Errorf("agent a 的 finding 应为 f-a: got %v", byAgent)
	}
	if byAgent["b"] != "f-b" {
		t.Errorf("agent b 的 finding 应为 f-b: got %v", byAgent)
	}
}

// TestGetFindingsAgentAllSeverityFilter 验证聚合模式下 severity 过滤仍生效:
// 两 agent 各有 1 条 finding(a=high, b=low),?severity=high 应只返回 a 的 finding。
func TestGetFindingsAgentAllSeverityFilter(t *testing.T) {
	dir := t.TempDir()
	s := newTwoAgentTestServer(t, dir)
	now := time.Now()
	recA := history.ScanRecord{
		ID:        "2026-01-01-00-00-00-aaaaaaaa",
		AgentID:   "a",
		StartedAt: now.Add(-1 * time.Minute),
		Scope:     "global",
		Findings: []security.Finding{
			{ID: "f-a", AgentID: "a", Severity: security.SeverityHigh, AssetID: "asset-a"},
		},
	}
	recB := history.ScanRecord{
		ID:        "2026-01-01-00-00-00-bbbbbbbb",
		AgentID:   "b",
		StartedAt: now,
		Scope:     "global",
		Findings: []security.Finding{
			{ID: "f-b", AgentID: "b", Severity: security.SeverityLow, AssetID: "asset-b"},
		},
	}
	s.History.Save(recA)
	s.History.Save(recB)

	w := reqDashboard(t, s, "GET", "/api/findings?agent=all&severity=high")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	var findings []security.Finding
	json.NewDecoder(w.Body).Decode(&findings)
	if len(findings) != 1 {
		t.Fatalf("severity=high 应只返回 1 条: got %d", len(findings))
	}
	if findings[0].ID != "f-a" {
		t.Errorf("应返回 f-a: got %s", findings[0].ID)
	}
}

// TestGetHealthAgentAll 验证 ?agent=all 返回 per-agent 健康分聚合对象:
// is_aggregate=true + agent_scores 数组,每项含 agent_id/agent_name/health_score。
// 不计算跨 agent 总分(健康分公式不跨 agent 聚合)。
func TestGetHealthAgentAll(t *testing.T) {
	dir := t.TempDir()
	s := newTwoAgentTestServer(t, dir)
	now := time.Now()
	recA := history.ScanRecord{
		ID:          "2026-01-01-00-00-00-aaaaaaaa",
		AgentID:     "a",
		StartedAt:   now.Add(-1 * time.Minute),
		Scope:       "global",
		HealthScore: &security.HealthScore{Score: 80, Band: "fair"},
	}
	recB := history.ScanRecord{
		ID:          "2026-01-01-00-00-00-bbbbbbbb",
		AgentID:     "b",
		StartedAt:   now,
		Scope:       "global",
		HealthScore: &security.HealthScore{Score: 90, Band: "good"},
	}
	s.History.Save(recA)
	s.History.Save(recB)

	w := reqDashboard(t, s, "GET", "/api/health?agent=all")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["is_aggregate"] != true {
		t.Errorf("应 is_aggregate=true: got %v", body["is_aggregate"])
	}
	scores, ok := body["agent_scores"].([]any)
	if !ok {
		t.Fatalf("应返回 agent_scores 数组: got %T", body["agent_scores"])
	}
	if len(scores) != 2 {
		t.Errorf("双 agent 应返回 2 条 agent_scores: got %d", len(scores))
	}
	byAgent := map[string]float64{}
	for _, sc := range scores {
		m, _ := sc.(map[string]any)
		id, _ := m["agent_id"].(string)
		hs, _ := m["health_score"].(map[string]any)
		score, _ := hs["score"].(float64)
		byAgent[id] = score
	}
	if byAgent["a"] != 80 {
		t.Errorf("agent a score 应为 80: got %v", byAgent["a"])
	}
	if byAgent["b"] != 90 {
		t.Errorf("agent b score 应为 90: got %v", byAgent["b"])
	}
}

// TestGetHealthAgentAllNoScan 验证聚合模式下某 agent 无扫描记录时,
// 该 agent 回退到 ComputeHealth(its assets, nil) 评分(与单 agent no-scan 路径一致)。
func TestGetHealthAgentAllNoScan(t *testing.T) {
	dir := t.TempDir()
	s := newTwoAgentTestServer(t, dir)
	// 不注入任何历史;两 agent 都应回退到 ComputeHealth(assets, nil) → score=100(无 finding)
	w := reqDashboard(t, s, "GET", "/api/health?agent=all")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["is_aggregate"] != true {
		t.Errorf("应 is_aggregate=true: got %v", body["is_aggregate"])
	}
	scores, _ := body["agent_scores"].([]any)
	if len(scores) != 2 {
		t.Errorf("应返回 2 条 agent_scores: got %d", len(scores))
	}
	for _, sc := range scores {
		m, _ := sc.(map[string]any)
		hs, _ := m["health_score"].(map[string]any)
		score, _ := hs["score"].(float64)
		if score != 100 {
			t.Errorf("无扫描的 agent 应回退 100: got %v", score)
		}
	}
}
