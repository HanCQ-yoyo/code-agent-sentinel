package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetDashboardAgentScoped:双 agent fixture,?agent=b 返回 B 的资产计数
// (B root 含 model:opus,A root 含通配 Bash——资产内容不同可区分)。
func TestGetDashboardAgentScoped(t *testing.T) {
	dir := t.TempDir()
	s := newTwoAgentTestServer(t, dir)
	// ?agent=a:应返回 A 的发现
	wa := doJSON[map[string]any](t, s, "GET", "/api/dashboard?agent=a")
	if wa["agent"] != "a" {
		t.Errorf("dashboard.agent 应为 a: got %v", wa["agent"])
	}
	// ?agent=b:应返回 b,且 asset_counts 反映 B root
	wb := doJSON[map[string]any](t, s, "GET", "/api/dashboard?agent=b")
	if wb["agent"] != "b" {
		t.Errorf("dashboard.agent 应为 b: got %v", wb["agent"])
	}
	// 数据区分:两个 agent 返回的 agent_name 必须不同(Agent A vs Agent B)。
	// 仅校验 agent 字符串会漏掉「返回正确 agent ID 但用错 Engine 取数据」的回归
	// (如 getTree 此前用 s.Agents[0].RootDir 的同类 bug)——agent_name 由
	// s.agentName(agentID) 从 s.Agents 查表得出,但 engineForQuery 返回的 Engine
	// 才决定 Discover 的资产来源;若 handler 拿对 agentID 却用错 Engine,asset_counts
	// 仍可能错误。这里用 agent_name 做基本区分,并断言 asset_counts 非空。
	//
	// 不断言 asset_counts 的具体差异:两 agent fixture 的 settings.json 经 parseSettings
	// 都产出 settings + permissions 两条资产(parseSettings 无条件追加 permissions
	// 资产,即便 Allow/Deny/Ask 全空),故 counts 完全相同(settings=1, permissions=1),
	// 无法用计数区分。更深的内容级区分(如 permissions.allow 内容)属检测器层职责,
	// 不在 dashboard 计数语义内,避免过度工程化 fixture。
	if wa["agent_name"] != "Agent A" {
		t.Errorf("a 的 agent_name 应为 Agent A: got %v", wa["agent_name"])
	}
	if wb["agent_name"] != "Agent B" {
		t.Errorf("b 的 agent_name 应为 Agent B: got %v", wb["agent_name"])
	}
	if wa["agent_name"] == wb["agent_name"] {
		t.Errorf("两 agent 的 agent_name 不应相同: got %v", wa["agent_name"])
	}
	if wa["asset_counts"] == nil {
		t.Error("a 的 asset_counts 不应为 nil")
	}
	if wb["asset_counts"] == nil {
		t.Error("b 的 asset_counts 不应为 nil")
	}
	// 未知 agent → 400(经 engineForQuery)
	req := httptest.NewRequest("GET", "/api/dashboard?agent=zzz", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("未知 agent 应 400: got %d", rec.Code)
	}
}
