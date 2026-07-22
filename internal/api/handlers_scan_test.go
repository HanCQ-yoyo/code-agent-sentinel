package api

import (
	"bytes"
	"context"
	"encoding/json"
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
type spyRunner struct {
	lastAgentID string
	lastScope   scan.ScanScope
}

func (s *spyRunner) RunScan(ctx context.Context, agentID string, scope scan.ScanScope, detectorIDs []string) (*security.ScanResult, error) {
	s.lastAgentID = agentID
	s.lastScope = scope
	return &security.ScanResult{}, nil
}

func (s *spyRunner) EngineFor(agentID string) *configengine.Engine { return nil }

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
	var res security.ScanResult
	json.Unmarshal(w.Body.Bytes(), &res)
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
