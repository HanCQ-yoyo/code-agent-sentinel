package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/security"
)

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
