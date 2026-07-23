package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/scan"
	"code-agent-sentinel/internal/security"
)

// reqScan 是扫描路由测试辅助:发请求(可选 body),返回 recorder。
// 路径含 query string(如 /api/scan?agent=foo),不带 body 时 body=nil。
func reqScan(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	r := s.Router()
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, path, nil)
	} else {
		b, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestPostScanPassesAgentQuery 验证 postScan 读 ?agent= 并传给 RunScan。
// 用 spyRunner 替换 s.Runner,断言收到的 agentID 与 query 一致。
func TestPostScanPassesAgentQuery(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	spy := &spyRunner{}
	s.Runner = spy
	w := reqScan(t, s, "POST", "/api/scan?agent=claude-code", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	if spy.lastAgentID != "claude-code" {
		t.Errorf("应传 agent=claude-code: got %q", spy.lastAgentID)
	}
}

// spyRunner 记录 RunScan 收到的 agentID/scope,满足 ScanRunner 接口。
// callCount 记录调用次数;batchIDs 记录每次调用收到的 batchID(多 agent 共享 batchID 验证用);
// failOnAgent 非空时,对该 agent ID 返回 error(部分失败测试用)。
type spyRunner struct {
	lastAgentID string
	lastScope   scan.ScanScope
	lastBatchID string
	callCount   int
	batchIDs    []string
	failOnAgent string
}

func (s *spyRunner) RunScan(ctx context.Context, agentID string, scope scan.ScanScope, detectorIDs []string, batchID string) (*security.ScanResult, error) {
	s.lastAgentID = agentID
	s.lastScope = scope
	s.lastBatchID = batchID
	s.callCount++
	s.batchIDs = append(s.batchIDs, batchID)
	if s.failOnAgent != "" && agentID == s.failOnAgent {
		return nil, fmt.Errorf("scan failed for %s", agentID)
	}
	return &security.ScanResult{}, nil
}

func (s *spyRunner) EngineFor(agentID string) *configengine.Engine { return nil }

// TestPostScanScopeQuery 验证 postScan 读 ?scope=/?path= 并构造 ScanScope 传 RunScan。
// project/asset 需 path,未知 scope → 400,缺省 global。
func TestPostScanScopeQuery(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	spy := &spyRunner{}
	s.Runner = spy
	// scope=project&path=/p
	w := reqScan(t, s, "POST", "/api/scan?agent=claude-code&scope=project&path=/p", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d %s", w.Code, w.Body)
	}
	if spy.lastScope.Type != "project" || spy.lastScope.Path != "/p" {
		t.Errorf("scope 应为 project /p: got %+v", spy.lastScope)
	}
	// 无 scope → global
	w2 := reqScan(t, s, "POST", "/api/scan", nil)
	if w2.Code != http.StatusOK {
		t.Fatalf("got %d", w2.Code)
	}
	if spy.lastScope.Type != "global" {
		t.Errorf("无 scope 应为 global: got %+v", spy.lastScope)
	}
	// scope=project 无 path → 400 bad_request
	w3 := reqScan(t, s, "POST", "/api/scan?scope=project", nil)
	if w3.Code != http.StatusBadRequest {
		t.Errorf("project 无 path 应 400: got %d %s", w3.Code, w3.Body)
	}
	// scope=bogus → 400 bad_scope
	w4 := reqScan(t, s, "POST", "/api/scan?scope=bogus", nil)
	if w4.Code != http.StatusBadRequest {
		t.Errorf("未知 scope 应 400: got %d %s", w4.Code, w4.Body)
	}
}

func TestPostScan(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), `{"permissions":{"allow":["Bash(*)"]}}`)
	s := newTestServer(t, dir)
	r := s.Router()
	req := httptest.NewRequest("POST", "/api/scan", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	// 新响应格式为 []AgentScanResult(多 agent 循环扫描);无参数单 agent 场景返回 1 元素数组。
	// 解码数组后取首元素验证 ScanResult 字段。
	var results []AgentScanResult
	json.Unmarshal(w.Body.Bytes(), &results)
	if len(results) != 1 {
		t.Fatalf("无参数单 agent 应返回 1 条结果: got %d", len(results))
	}
	res := results[0]
	if len(res.Findings) == 0 {
		t.Error("应检出通配 Bash")
	}
	if res.HealthScore == nil {
		t.Error("应返回健康分")
	}
	// 扫描后历史目录应至少有 1 条记录
	list := doJSON[[]map[string]any](t, s, "GET", "/api/history")
	if len(list) == 0 {
		t.Error("POST /scan 后应持久化一条历史记录")
	}
}

func TestGetScanResultRestoresLatest(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), `{"model":"opus"}`)
	s := newTestServer(t, dir)
	r := s.Router()
	// 先扫描一次
	req := httptest.NewRequest("POST", "/api/scan", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("scan got %d", w.Code)
	}
	// GET /scan/result 应返回最近一条(非空 {})
	greq := httptest.NewRequest("GET", "/api/scan/result", nil)
	greq.Host = "127.0.0.1"
	greq.Header.Set("Authorization", "Bearer tok")
	gw := httptest.NewRecorder()
	r.ServeHTTP(gw, greq)
	if gw.Code != 200 {
		t.Fatalf("got %d", gw.Code)
	}
	body := gw.Body.String()
	if body == "{}" || body == "" {
		t.Fatalf("应返回最近一条扫描结果,got %q", body)
	}
}

func TestHistoryRoutes(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), `{"permissions":{"allow":["Bash(*)"]}}`)
	s := newTestServer(t, dir)
	// 扫描产生一条历史
	doJSON[map[string]any](t, s, "POST", "/api/scan")

	list := doJSON[[]map[string]any](t, s, "GET", "/api/history")
	if len(list) != 1 {
		t.Fatalf("应有 1 条历史,got %d", len(list))
	}
	id, _ := list[0]["id"].(string)
	if id == "" {
		t.Fatal("历史摘要缺 id")
	}

	detail := doJSON[map[string]any](t, s, "GET", "/api/history/"+id)
	if detail["id"] != id {
		t.Fatalf("详情 id 不匹配: %v", detail["id"])
	}
	if _, ok := detail["findings"]; !ok {
		t.Error("详情应含 findings")
	}
	if _, ok := detail["inventory"]; !ok {
		t.Error("详情应含 inventory 快照")
	}

	// 删除
	doJSON[map[string]any](t, s, "DELETE", "/api/history/"+id)
	list2 := doJSON[[]map[string]any](t, s, "GET", "/api/history")
	if len(list2) != 0 {
		t.Fatalf("删除后应为空,got %d", len(list2))
	}
}

// doJSON 是路由测试辅助:发请求,200 解 JSON。
func doJSON[T any](t *testing.T, s *Server, method, path string) T {
	t.Helper()
	r := s.Router()
	req := httptest.NewRequest(method, path, nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("%s %s: got %d: %s", method, path, w.Code, w.Body)
	}
	var v T
	json.Unmarshal(w.Body.Bytes(), &v)
	return v
}

func TestSaveHistoryProjects(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), `{"model":"opus"}`)
	// 登记一个项目触发 discoverProjects
	cj := filepath.Join(dir, ".claude.json")
	writeFile(t, cj, `{"projects":{"`+filepath.Join(dir, "myproj")+`":{}}}`)
	writeFile(t, filepath.Join(dir, "myproj", ".claude", "settings.json"), `{"model":"sonnet"}`)

	s := newTestServer(t, dir)
	r := s.Router()
	req := httptest.NewRequest("POST", "/api/scan", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("scan got %d: %s", w.Code, w.Body)
	}
	// 取历史详情,断言 projects 非空
	req2 := httptest.NewRequest("GET", "/api/history", nil)
	req2.Host = "127.0.0.1"
	req2.Header.Set("Authorization", "Bearer tok")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("history got %d", w2.Code)
	}
	// 取最新详情(列表第一项 id)
	var list []map[string]any
	json.Unmarshal(w2.Body.Bytes(), &list)
	if len(list) == 0 {
		t.Skip("无历史记录")
	}
	id := list[0]["id"].(string)
	req3 := httptest.NewRequest("GET", "/api/history/"+id, nil)
	req3.Host = "127.0.0.1"
	req3.Header.Set("Authorization", "Bearer tok")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	var rec map[string]any
	json.Unmarshal(w3.Body.Bytes(), &rec)
	projects, ok := rec["projects"]
	if !ok {
		t.Fatal("ScanRecord 应有 projects 字段")
	}
	if pl, _ := projects.([]any); len(pl) == 0 {
		t.Error("projects 不应为空(已登记项目)")
	}
}

// TestPostScanMultiAgents 验证 POST /api/scan?agents=a,b 循环扫多 agent:
// spy 被调用 2 次(每 agent 一次),两次 batchID 相同(共享),响应是 2 元素数组。
// 用 newTwoAgentTestServer 提供 ID 为 a/b 的两个真实 agent(单 agent fixture 无 agent-b)。
func TestPostScanMultiAgents(t *testing.T) {
	dir := t.TempDir()
	s := newTwoAgentTestServer(t, dir)
	spy := &spyRunner{}
	s.Runner = spy
	w := reqScan(t, s, "POST", "/api/scan?agents=a,b", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	if spy.callCount != 2 {
		t.Errorf("应调 2 次 RunScan: got %d", spy.callCount)
	}
	// 两次调用应共享同一 batchID
	if len(spy.batchIDs) == 2 && spy.batchIDs[0] != spy.batchIDs[1] {
		t.Errorf("多 agent 应共享 batchID: got %q vs %q", spy.batchIDs[0], spy.batchIDs[1])
	}
	// 响应是数组,长度 2
	var results []AgentScanResult
	json.NewDecoder(w.Body).Decode(&results)
	if len(results) != 2 {
		t.Errorf("应返回 2 条结果: got %d", len(results))
	}
}

// TestPostScanMultiAgents_OneFails 验证部分失败仍返回 200,
// 失败 agent 在数组中带 error 字段,成功 agent 无 error。
func TestPostScanMultiAgents_OneFails(t *testing.T) {
	dir := t.TempDir()
	s := newTwoAgentTestServer(t, dir)
	spy := &spyRunner{failOnAgent: "b"}
	s.Runner = spy
	w := reqScan(t, s, "POST", "/api/scan?agents=a,b", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("部分失败仍应 200: %d", w.Code)
	}
	var results []AgentScanResult
	json.NewDecoder(w.Body).Decode(&results)
	if len(results) != 2 {
		t.Fatalf("应返回 2 条结果: got %d", len(results))
	}
	for _, r := range results {
		if r.AgentID == "b" && r.Error == "" {
			t.Error("b 应有 error")
		}
		if r.AgentID == "a" && r.Error != "" {
			t.Error("a 不应有 error")
		}
	}
}
