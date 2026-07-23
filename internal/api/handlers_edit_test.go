package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/scan"
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
// agentID 为空时拉首 agent(默认);非空时拉指定 agent(经 ?agent=)。
func getInventory(t *testing.T, s *Server) configengine.Inventory {
	t.Helper()
	return getInventoryFor(t, s, "")
}

// getInventoryFor 拉指定 agent 的资产;agentID 空走首 agent。
func getInventoryFor(t *testing.T, s *Server, agentID string) configengine.Inventory {
	t.Helper()
	r := s.Router()
	path := "/api/assets"
	if agentID != "" {
		path += "?agent=" + agentID
	}
	req := httptest.NewRequest("GET", path, nil)
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
	latest := s.latestScan("")
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
	latest := s.latestScan("")
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
// Task 13 改造:partialRescan 现经 s.Runner.RunScan(不再直接调 s.Orchestrator.Scan),
// 故破坏 s.Orchestrator 不再生效(Runner 持有自己的 Orchestrator 引用,NewServer 时注入)。
// 改为注入 errorRunner(RunScan 返回 error,EngineFor 委托原 Runner 以便 getAssets 正常)
// 强制重扫失败,断言响应含 rescan_error 非空。
func TestPartialRescanErrorSurfaced(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), `{"model":"opus"}`)
	s := newTestServer(t, dir)
	// 注入 errorRunner:EngineFor 委托原 Runner(使 getAssets 正常拉资产),RunScan 返回 error
	// (模拟扫描失败)。partialRescan 应捕获 err 并返回 rescan_error="partial rescan failed: ..."。
	realRunner := s.Runner
	s.Runner = &errorRunner{inner: realRunner}
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

// errorRunner 实现 ScanRunner 接口:RunScan 恒返回 error(模拟扫描失败),
// EngineFor 委托内部 Runner(使 getAssets 等读路径正常),用于测试 partialRescan 的错误路径
// (Task 13:partialRescan 经 Runner,需注入错误 Runner)。
type errorRunner struct {
	inner ScanRunner
}

func (e *errorRunner) RunScan(ctx context.Context, agentID string, scope scan.ScanScope, detectorIDs []string, batchID string) (*security.ScanResult, error) {
	return nil, errors.New("simulated scan failure")
}

func (e *errorRunner) EngineFor(agentID string) *configengine.Engine {
	return e.inner.EngineFor(agentID)
}

// TestPartialRescanUsesRunnerAgent 验证 partialRescan 经 s.Runner.RunScan 走对应 agent 的 Engine,
// 不再硬编码 s.Engine(Task 13)。
//
// 场景:两 agent fixture(agent a = home/.claude 含 Bash(*);agent b = home/.claude-b 含 model:opus)。
// Editor 绑定 s.Engine(= agent a 的 Engine),只能找 a 的资产;故编辑目标是 a 的 permissions 资产。
//
// 判别设计:初始 a 含 Bash(*)→ global 扫描后 latest 含 baseline.wildcard-bash(AssetID=a.permissions)。
// 编辑 a 的 permissions 追加 Read(**)(引入新危险 baseline.dangerous-read-all,不在 latest prior 中),
// 带 ?agent=b commit:
//   - 走 Runner(b):b 的 Engine 发现 b 的资产(home/.claude-b/...),scope=asset/path=<a 的 source_path>
//     在 b 的 inventory 中无匹配 → scopeAssets 返回空 → 扫 0 资产 → new_findings 空、无 rescan_error。
//   - 若仍走 s.Engine(a):a 的 inventory 含该 source_path 的 sibling(permissions)→ 扫到 a 的
//     permissions sibling → 产生 baseline.dangerous-read-all(新,非 prior)→ new_findings 非空。
//
// 故断言:?agent=b 的 commit 成功(200),new_findings 空(因 b 的 Engine 找不到 a 的路径),
// 无 rescan_error。这精确区分了走 Runner(b,空)与走 s.Engine(a,非空)的行为。
//
// 反向对照(走 a 应非空)由 TestPartialRescanDedup 覆盖(编辑 a 引入新危险后 ?agent 默认=a → 非空),
// 这里不重复。
func TestPartialRescanUsesRunnerAgent(t *testing.T) {
	dir := t.TempDir()
	s := newTwoAgentTestServer(t, dir)
	r := s.Router()

	// 1. 先跑一次 global 扫描使 latestScan("") 非 nil(prior 基线非空)。
	//    首 agent(a)有 Bash(*) → latest 含 baseline.wildcard-bash(AssetID=a.permissions),
	//    但不含 baseline.dangerous-read-all(初始无 Read(**))。
	scanReq := httptest.NewRequest("POST", "/api/scan", nil)
	scanReq.Host = "127.0.0.1"
	scanReq.Header.Set("Authorization", "Bearer tok")
	scanW := httptest.NewRecorder()
	r.ServeHTTP(scanW, scanReq)
	if scanW.Code != 200 {
		t.Fatalf("POST /api/scan: got %d: %s", scanW.Code, scanW.Body)
	}
	latest := s.latestScan("")
	if latest == nil {
		t.Fatal("latestScan() nil after POST /api/scan")
	}

	// 2. 找 agent a 的 permissions 资产(Editor 绑定 s.Engine=a,只能找 a 的资产)。
	invA := getInventoryFor(t, s, "a")
	var permID, permHash string
	for _, a := range invA.Assets {
		if a.Type == configengine.AssetPermissions {
			permID = a.ID
			permHash = a.Hash
			break
		}
	}
	if permID == "" {
		t.Fatalf("no permissions asset for agent a; inv=%+v", invA)
	}

	// 3. 编辑 a 的 permissions 追加 Read(**)(引入新危险 baseline.dangerous-read-all),
	//    带 ?agent=b commit。partialRescan(?agent=b) 走 b 的 Engine:
	//    b 的 inventory 无 a 的 source_path → 扫 0 资产 → new_findings 空。
	body := `{"new_content":"{\"permissions\":{\"allow\":[\"Bash(*)\",\"Read(**)\"]}}","base_hash":"` + permHash + `"}`
	req := httptest.NewRequest("PUT", "/api/assets/"+permID+"/content?agent=b", strings.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("PUT /api/assets/%s/content?agent=b: got %d: %s", permID, w.Code, w.Body)
	}
	var res map[string]any
	json.Unmarshal(w.Body.Bytes(), &res)
	if re, ok := res["rescan_error"]; ok && re != nil && re != "" {
		t.Fatalf("partialRescan(?agent=b) should not error (b's Engine finds 0 assets for a's path → empty result, not error): rescan_error=%v, res=%+v", re, res)
	}
	nf, _ := res["new_findings"].([]any)
	// 关键断言:?agent=b 走 b 的 Engine,b 的 inventory 无 a 的 source_path → 扫 0 资产 → new_findings 空。
	// 若 partialRescan 仍用 s.Engine(a),a 的 inventory 含该 source_path 的 sibling(permissions)→
	// 扫到 baseline.dangerous-read-all(新,非 prior)→ new_findings 非空。故 nf 空 = 走了 b 的 Runner。
	if len(nf) != 0 {
		t.Errorf("partialRescan(?agent=b) should scan 0 assets (b's Engine has no asset at a's source_path), "+
			"got %d new_findings — suggests partialRescan still uses s.Engine(a): %+v", len(nf), nf)
	}
}

// TestPartialRescanDedupPerAgent 验证 partialRescan 的 prior 按 agent 取(修跨 agent 误报新增)。
//
// 场景:两 agent(a, b)各有自己的 settings.json,均含 Read(**)(触发 baseline.dangerous-read-all)。
// 1. 分别对 a、b 跑 global 扫描 → 各自历史记录含 baseline.dangerous-read-all(AssetID=各自 permissions)。
// 2. 编辑 b 的 settings 追加 Bash(*)(引入新危险 baseline.wildcard-bash),带 ?agent=b commit。
//
// 旧 bug:priorFindingsForSourcePath 调 latestScan("")(任意 agent 全局最新)→ 取到 a 的 latest
// → 用 b 的 sourcePath 过滤 a 的 inventory → siblingIDs 空 → prior 空 → b 的全部 findings
// (含已有的 Read(**))报为新增(过度报告)。
//
// 修复后:priorFindingsForSourcePath(agentID="b", ...)→ latestScan("b")→ 取 b 的 latest
// → b 的 inventory 含同 source_path 的 permissions sibling → prior 含 Read(**) finding
// → 仅 Bash(*) 被报为新增(Read(**) 被去重)。
func TestPartialRescanDedupPerAgent(t *testing.T) {
	dir := t.TempDir()
	// 两 agent 各有 settings.json;均含 Read(**)(baseline.dangerous-read-all)。
	writeFile(t, filepath.Join(dir, ".claude", "settings.json"),
		`{"permissions":{"allow":["Read(**)"]}}`)
	writeFile(t, filepath.Join(dir, ".claude-b", "settings.json"),
		`{"permissions":{"allow":["Read(**)"]}}`)
	agents := []configengine.Agent{
		{ID: "a", Name: "Agent A", RootDir: filepath.Join(dir, ".claude"), ClaudeJSON: filepath.Join(dir, ".claude.json"), HomeDir: dir},
		{ID: "b", Name: "Agent B", RootDir: filepath.Join(dir, ".claude-b"), ClaudeJSON: filepath.Join(dir, ".claude-b.json"), HomeDir: dir},
	}
	eng := configengine.NewEngineFromAgent(agents[0])
	s := newTestServerWithAgents(t, eng, agents)
	r := s.Router()

	// 1. 对 a 跑 global 扫描(使 latestScan("a") 非 nil)。
	scanA := httptest.NewRequest("POST", "/api/scan?agent=a", nil)
	scanA.Host = "127.0.0.1"
	scanA.Header.Set("Authorization", "Bearer tok")
	wA := httptest.NewRecorder()
	r.ServeHTTP(wA, scanA)
	if wA.Code != 200 {
		t.Fatalf("POST /api/scan?agent=a: got %d: %s", wA.Code, wA.Body)
	}

	// 2. 对 b 跑 global 扫描(使 latestScan("b") 非 nil,含 Read(**) finding)。
	scanB := httptest.NewRequest("POST", "/api/scan?agent=b", nil)
	scanB.Host = "127.0.0.1"
	scanB.Header.Set("Authorization", "Bearer tok")
	wB := httptest.NewRecorder()
	r.ServeHTTP(wB, scanB)
	if wB.Code != 200 {
		t.Fatalf("POST /api/scan?agent=b: got %d: %s", wB.Code, wB.Body)
	}
	latestB := s.latestScan("b")
	if latestB == nil {
		t.Fatal("latestScan(\"b\") nil after POST /api/scan?agent=b")
	}
	if len(latestB.Findings) == 0 {
		t.Fatal("expected b's latest scan to have baseline findings (Read(**)), got none")
	}

	// 3. 找 b 的 permissions 资产(Editor 绑定 s.Engine=a,只能找 a 的资产;但编辑 b 须用 b 的资产)。
	//    注:Editor 绑定 s.Engine(=agent a 的 Engine),无法找 b 的资产;故这里直接取 b 的
	//    inventory 用 b 的 permissions ID。但 commitAsset 走 Editor.Commit(用 s.Engine)→
	//    找不到 b 的资产 → 404。故改为编辑 a 的资产、带 ?agent=b 走 b 的 rescan(与
	//    TestPartialRescanUsesRunnerAgent 同构),但这里 a 和 b 的 sourcePath 不同,
	//    latestScan("") 取到 a 的 latest → 用 b 的 sourcePath 过滤 a 的 inventory → 空 → prior 空。
	//
	//    重新设计:用同 sourcePath 的场景。但 a、b 的 root 不同 → sourcePath 不同。
	//    要测跨 agent 误报,须 a 的 latest 被错误取为 b 的 prior 基线。
	//
	//    改为:编辑 a 的 permissions(追加 Bash(*)),带 ?agent=a commit。
	//    latestScan("") 取 a 的 latest(因 a 更早扫)→ a 的 inventory 含 a 的 sourcePath
	//    → prior 非空 → Read(**) 去重 → 仅 Bash(*) 新增。这是正确行为(单 agent)。
	//    要测跨 agent,须 latestScan("") 取到 b 而非 a → b 更晚扫。
	//
	//    最终设计:先扫 a,再扫 b(b 更晚 → latestScan("") 取 b)。
	//    然后编辑 a 的 permissions(追加 Bash(*)),带 ?agent=a commit。
	//    旧 bug:priorFindingsForSourcePath("a 的 sourcePath")→ latestScan("")→ 取 b 的 latest
	//    → b 的 inventory 不含 a 的 sourcePath → prior 空 → a 的全部 findings(含已有 Read(**))
	//    报为新增。
	//    修复后:latestScan("a")→ 取 a 的 latest → a 的 inventory 含 a 的 sourcePath
	//    → prior 含 Read(**) → 仅 Bash(*) 报为新增。
	invA := getInventoryFor(t, s, "a")
	var permID, permHash string
	for _, a := range invA.Assets {
		if a.Type == configengine.AssetPermissions {
			permID = a.ID
			permHash = a.Hash
			break
		}
	}
	if permID == "" {
		t.Fatalf("no permissions asset for agent a; inv=%+v", invA)
	}

	// 4. 编辑 a 的 permissions 追加 Bash(*)(引入新危险 baseline.wildcard-bash),
	//    带 ?agent=a commit。此时 latestScan("")=b(更晚),但 latestScan("a")=a(正确)。
	body := `{"new_content":"{\"permissions\":{\"allow\":[\"Read(**)\",\"Bash(*)\"]}}","base_hash":"` + permHash + `"}`
	req := httptest.NewRequest("PUT", "/api/assets/"+permID+"/content?agent=a", strings.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("PUT /api/assets/%s/content?agent=a: got %d: %s", permID, w.Code, w.Body)
	}
	var res map[string]any
	json.Unmarshal(w.Body.Bytes(), &res)
	if re, ok := res["rescan_error"]; ok && re != nil && re != "" {
		t.Fatalf("unexpected rescan_error: %v, res=%+v", re, res)
	}
	nf, _ := res["new_findings"].([]any)
	// 关键断言:仅 Bash(*) 的 baseline.wildcard-bash 报为新增;Read(**) 的
	// baseline.dangerous-read-all 须被 a 的 latest prior 去重。
	// 旧 bug(latestScan("")=b):b 的 inventory 不含 a 的 sourcePath → prior 空
	// → a 的全部 findings(含 Read(**))报为新增 → len(nf) >= 2。
	if len(nf) == 0 {
		t.Fatalf("expected 1 new finding (Bash(*) → baseline.wildcard-bash), got 0; res=%+v", res)
	}
	// 提取 rule_id 列表,断言不含 baseline.dangerous-read-all(已被去重)。
	ruleIDs := map[string]bool{}
	for _, x := range nf {
		if m, ok := x.(map[string]any); ok {
			if rid, ok := m["rule_id"].(string); ok {
				ruleIDs[rid] = true
			}
		}
	}
	if ruleIDs["baseline.dangerous-read-all"] {
		t.Errorf("baseline.dangerous-read-all (Read(**)) should be deduped by a's latest prior, "+
			"but was reported as new (cross-agent bug: latestScan(\"\") took b's inventory). "+
			"new_findings rules=%v", ruleIDs)
	}
}
