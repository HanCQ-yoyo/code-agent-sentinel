package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security"
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

// TestPartialRescanDedupSettingsEdit 验证编辑 SETTINGS 资产(非 permissions)时,
// 同 source_path 的 permissions sibling 的既有 finding 不被误报为新增(Fix 3)。
//
// 旧 bug:partialRescan 的 prior 只用 updated.ID(= settings.ID),但 baseline
// findings 的 AssetID = permissions.ID(sibling),prior 为空 → 全部 sibling findings
// 报为"新增"(包括编辑前就存在的)→ 过度报告。
//
// 修复:priorFindingsForSourcePath 收集同 source_path 全部 sibling AssetID 过滤。
// 本测试:BEFORE 有 Read(**) → baseline.dangerous-read-all(AssetID=permissions.ID);
// 编辑 settings(不改 permissions 内容,如改 model)→ new_findings 须为空(无新增)。
func TestPartialRescanDedupSettingsEdit(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	// BEFORE:Read(**) 触发 baseline.dangerous-read-all(AssetID=permissions.ID)。
	writeFile(t, filepath.Join(claude, "settings.json"),
		`{"permissions":{"allow":["Read(**)"]},"model":"opus"}`)
	s := newTestServer(t, dir)
	r := s.Router()

	// 1. 全量扫描,使 latestScan() 非 nil。
	scanReq := httptest.NewRequest("POST", "/api/scan", nil)
	scanReq.Host = "127.0.0.1"
	scanReq.Header.Set("Authorization", "Bearer tok")
	scanW := httptest.NewRecorder()
	r.ServeHTTP(scanW, scanReq)
	if scanW.Code != 200 {
		t.Fatalf("POST /api/scan: got %d: %s", scanW.Code, scanW.Body)
	}
	latest := s.latestScan()
	if latest == nil || len(latest.Findings) == 0 {
		t.Fatal("expected baseline findings in latest scan")
	}

	// 2. 找 settings 资产(非 permissions)。
	inv := getInventory(t, s)
	var settingsID, settingsHash string
	for _, a := range inv.Assets {
		if a.Type == configengine.AssetSettings {
			settingsID = a.ID
			settingsHash = a.Hash
			break
		}
	}
	if settingsID == "" {
		t.Fatal("no settings asset found")
	}

	// 3. 编辑 settings 的 model 字段(不动 permissions)→ 不应引入新 finding。
	//    permissions sibling 的 Read(**) finding 已在 latest 中,prior 须含它 → 去重。
	body := `{"new_content":"{\"permissions\":{\"allow\":[\"Read(**)\"]},\"model\":\"sonnet\"}","base_hash":"` + settingsHash + `"}`
	req := httptest.NewRequest("PUT", "/api/assets/"+settingsID+"/content", strings.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("PUT settings: got %d: %s", w.Code, w.Body)
	}
	var res map[string]any
	json.Unmarshal(w.Body.Bytes(), &res)
	nf, _ := res["new_findings"].([]any)
	// 旧 bug:permissions 的 Read(**) finding 被误报为新增(因为 prior 用 settings.ID,
	// 而 finding 的 AssetID 是 permissions.ID)。
	if len(nf) > 0 {
		t.Fatalf("expected NO new_findings (permissions finding is pre-existing, not new), got %d: %+v", len(nf), nf)
	}
	// Fix 5:new_findings 须是 [] 非 null。
	if res["new_findings"] == nil {
		t.Fatal("Fix 5: new_findings should be [] not null")
	}
}

// TestPartialRescanErrorSurfaced 验证重扫失败时 rescan_error 字段被设置(Fix 4)。
// 旧 bug:partialRescan 失败返回 nil → new_findings: null → 前端误报"无新增风险"。
//
// 本测试通过让 Orchestrator.Registry 为 nil(无法 Scan)强制重扫失败,
// 断言响应含 rescan_error 非空字符串。
func TestPartialRescanErrorSurfaced(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), `{"model":"opus"}`)
	s := newTestServer(t, dir)
	// 破坏 Orchestrator 使 Scan 必失败:nil Registry → Detectors() panic 或 nil deref。
	// 用一个无 Registry 的 Orchestrator 替换。
	s.Orchestrator = &security.Orchestrator{Registry: nil}
	r := s.Router()
	inv := getInventory(t, s)
	id := inv.Assets[0].ID
	body := `{"new_content":"{\"model\":\"sonnet\"}","base_hash":"` + inv.Assets[0].Hash + `"}`
	req := httptest.NewRequest("PUT", "/api/assets/"+id+"/content", strings.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("commit should succeed (write ok), got %d: %s", w.Code, w.Body)
	}
	var res map[string]any
	json.Unmarshal(w.Body.Bytes(), &res)
	re, ok := res["rescan_error"]
	if !ok || re == nil || re == "" {
		t.Fatalf("expected non-empty rescan_error, got %+v", res)
	}
	// new_findings 仍须是 [](非 null),因 partialRescan 失败时返 make([]Finding,0)。
	nf, _ := res["new_findings"].([]any)
	if nf == nil {
		t.Fatal("Fix 5: new_findings should be [] not null even on rescan failure")
	}
}
