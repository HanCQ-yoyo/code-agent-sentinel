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

func TestCommitAssetWritesAndRescans(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	// 危险 permissions:Bash(*) 会让 baseline 报 finding
	writeFile(t, filepath.Join(claude, "settings.json"), `{"permissions":{"allow":["Bash(git:*)"]}}`)
	s := newTestServer(t, dir)
	r := s.Router()
	inv := getInventory(t, s)
	id := inv.Assets[0].ID
	// 编辑成更危险:Bash(*)
	body := `{"new_content":"{\"permissions\":{\"allow\":[\"Bash(*)\"]}}","base_hash":"` + inv.Assets[0].Hash + `"}`
	req := httptest.NewRequest("PUT", "/api/assets/"+id+"/content", strings.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	var res map[string]any
	json.Unmarshal(w.Body.Bytes(), &res)
	if res["backup_path"] == nil || res["backup_path"] == "" {
		t.Fatal("missing backup_path")
	}
	nf, _ := res["new_findings"].([]any)
	// Bash(*) 应触发 baseline finding(新增)
	if len(nf) == 0 {
		t.Fatalf("expected new_findings for Bash(*), got %+v", res)
	}
}

func TestCommitAssetConcurrentModification(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".claude", "settings.json"), `{"model":"opus"}`)
	s := newTestServer(t, dir)
	r := s.Router()
	inv := getInventory(t, s)
	id := inv.Assets[0].ID
	body := `{"new_content":"{\"model\":\"sonnet\"}","base_hash":"stale"}`
	req := httptest.NewRequest("PUT", "/api/assets/"+id+"/content", strings.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 409 {
		t.Fatalf("want 409 got %d: %s", w.Code, w.Body)
	}
}

func TestCommitAssetNotEditable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".claude.json"), `{"mcpServers":{"foo":{"command":"bar"}}}`)
	writeFile(t, filepath.Join(dir, ".claude", "settings.json"), `{"model":"opus"}`)
	s := newTestServer(t, dir)
	r := s.Router()
	inv := getInventory(t, s)
	var id string
	for _, a := range inv.Assets {
		if a.SourcePath == filepath.Join(dir, ".claude.json") {
			id = a.ID
		}
	}
	body := `{"new_content":"{}","base_hash":"x"}`
	req := httptest.NewRequest("PUT", "/api/assets/"+id+"/content", strings.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("want 403 got %d: %s", w.Code, w.Body)
	}
}

func TestCommitAssetBadContent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".claude", "settings.json"), `{"model":"opus"}`)
	s := newTestServer(t, dir)
	r := s.Router()
	inv := getInventory(t, s)
	id := inv.Assets[0].ID
	body := `{"new_content":"{not json","base_hash":"` + inv.Assets[0].Hash + `"}`
	req := httptest.NewRequest("PUT", "/api/assets/"+id+"/content", strings.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("want 400 got %d: %s", w.Code, w.Body)
	}
}

// TestPartialRescanDedup 验证部分重扫的去重逻辑在 latestScan() 非 nil 时正确工作。
//
// 场景:用户先跑全量扫描(latest 有基线 findings),再编辑引入新危险。
// 旧 bug:findingKey 用 Finding.ID(死字段,恒 ""),所有 finding key 均为 "",
// priorKeys={"":true} → 新 finding key "" 命中 → 误判「已存在」→ 被丢弃 →
// new_findings 恒空,部分重扫的核心价值(标出新危险)静默失效。
//
// 本测试用 Read(**)(已有)+ Bash(git:*)(安全)作为 BEFORE,Bash(*)(新危险)
// 作为 AFTER:BEFORE 只有 baseline.dangerous-read-all,AFTER 额外引入
// baseline.wildcard-bash。修复后后者应报为新增;旧 bug 下会被丢弃。
func TestPartialRescanDedup(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	// BEFORE: Bash(git:*) 不含子串 "Bash(*)" → 不触发 baseline.wildcard-bash;
	// Read(**) 触发 baseline.dangerous-read-all → latest 基线有 1 条 finding。
	writeFile(t, filepath.Join(claude, "settings.json"),
		`{"permissions":{"allow":["Bash(git:*)","Read(**)"]}}`)
	s := newTestServer(t, dir)
	r := s.Router()

	// 1. 先跑全量扫描,使 latestScan() 非 nil(prior 集合非空)。
	scanReq := httptest.NewRequest("POST", "/api/scan", nil)
	scanReq.Host = "127.0.0.1"
	scanReq.Header.Set("Authorization", "Bearer tok")
	scanW := httptest.NewRecorder()
	r.ServeHTTP(scanW, scanReq)
	if scanW.Code != 200 {
		t.Fatalf("POST /api/scan: got %d: %s", scanW.Code, scanW.Body)
	}
	latest := s.latestScan()
	if latest == nil {
		t.Fatal("latestScan() nil after POST /api/scan")
	}
	if len(latest.Findings) == 0 {
		t.Fatal("expected baseline findings in latest scan, got none")
	}

	// 2. 找 permissions 资产(baseline 规则匹配 permissions 类型,非 settings)。
	// settings 与 permissions 同 SourcePath、同 Hash(fillHash 均读同一文件)。
	inv := getInventory(t, s)
	var permID, permHash string
	for _, a := range inv.Assets {
		if a.Type == configengine.AssetPermissions {
			permID = a.ID
			permHash = a.Hash
			break
		}
	}
	if permID == "" {
		t.Fatal("no permissions asset found")
	}

	// 3. 编辑成更危险:Bash(*) 引入 baseline.wildcard-bash(新规则,新 finding)。
	body := `{"new_content":"{\"permissions\":{\"allow\":[\"Bash(*)\",\"Read(**)\"]}}","base_hash":"` + permHash + `"}`
	req := httptest.NewRequest("PUT", "/api/assets/"+permID+"/content", strings.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("PUT /api/assets/%s/content: got %d: %s", permID, w.Code, w.Body)
	}
	var res map[string]any
	json.Unmarshal(w.Body.Bytes(), &res)
	nf, _ := res["new_findings"].([]any)
	// Bash(*) 引入 baseline.wildcard-bash(新规则),不应被 Read(**) 的旧 finding 去重掉。
	if len(nf) == 0 {
		t.Fatalf("expected new_findings (Bash(*) is a new danger), got empty; dedup dropped it.\n"+
			"latest findings=%d, response=%+v", len(latest.Findings), res)
	}
}
