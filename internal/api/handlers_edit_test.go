package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

func TestPreviewAsset(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), `{"model":"opus"}`)
	s := newTestServer(t, dir)
	r := s.Router()
	// 查资产 ID
	inv := getInventory(t, s)
	id := inv.Assets[0].ID
	body := `{"new_content":"{\"model\":\"sonnet\"}","base_hash":"` + inv.Assets[0].Hash + `"}`
	req := httptest.NewRequest("POST", "/api/assets/"+id+"/preview", strings.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	var pr map[string]any
	json.Unmarshal(w.Body.Bytes(), &pr)
	if pr["editable"] != true {
		t.Fatalf("want editable true: %+v", pr)
	}
	if pr["base_hash_ok"] != true {
		t.Fatalf("want base_hash_ok true: %+v", pr)
	}
}

func TestPreviewAssetNotFound(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".claude", "settings.json"), `{"model":"opus"}`)
	s := newTestServer(t, dir)
	r := s.Router()
	req := httptest.NewRequest("POST", "/api/assets/nonexistent/preview", strings.NewReader(`{"new_content":"{}","base_hash":"x"}`))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("want 404 got %d: %s", w.Code, w.Body)
	}
}

func TestPreviewAssetNotEditable(t *testing.T) {
	dir := t.TempDir()
	// ~/.claude.json MCP 不可编辑
	writeFile(t, filepath.Join(dir, ".claude.json"), `{"mcpServers":{"foo":{"command":"bar"}}}`)
	writeFile(t, filepath.Join(dir, ".claude", "settings.json"), `{"model":"opus"}`)
	s := newTestServer(t, dir)
	r := s.Router()
	inv := getInventory(t, s)
	var id string
	for _, a := range inv.Assets {
		if a.SourcePath == filepath.Join(dir, ".claude.json") {
			id = a.ID
			break
		}
	}
	if id == "" {
		t.Fatal("no mcp asset")
	}
	req := httptest.NewRequest("POST", "/api/assets/"+id+"/preview", strings.NewReader(`{"new_content":"{}","base_hash":"x"}`))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// 不可编辑返 200 + editable:false(spec:前端据此禁用)
	if w.Code != 200 {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body)
	}
	var pr map[string]any
	json.Unmarshal(w.Body.Bytes(), &pr)
	if pr["editable"] != false {
		t.Fatalf("want editable false: %+v", pr)
	}
}

// getInventory 复用 GET /api/assets 拉资产(测试辅助)。
func getInventory(t *testing.T, s *Server) configengine.Inventory {
	t.Helper()
	r := s.Router()
	req := httptest.NewRequest("GET", "/api/assets", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var inv configengine.Inventory
	json.Unmarshal(w.Body.Bytes(), &inv)
	return inv
}
