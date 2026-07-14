package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

func TestGetTreeGlobal(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), `{"model":"opus"}`)
	writeFile(t, filepath.Join(claude, "skills", "injection", "SKILL.md"), `# skill`)
	s := newTestServer(t, dir)
	r := s.Router()
	req := httptest.NewRequest("GET", "/api/tree?scope=global", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	var root configengine.TreeNode
	json.Unmarshal(w.Body.Bytes(), &root)
	if root.Kind != "dir" {
		t.Errorf("根 Kind = %q, 期望 dir", root.Kind)
	}
	// settings.json 真实存在 + 挂资产
	found := false
	for _, c := range root.Children {
		if c.Path == "settings.json" {
			found = true
		}
	}
	if !found {
		t.Error("缺 settings.json 节点")
	}
}

func TestGetTreeProjectPathValidation(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), `{}`)
	// 登记一个已知项目
	projPath := filepath.Join(dir, "myproj")
	writeFile(t, filepath.Join(dir, ".claude.json"), `{"projects":{"`+projPath+`":{}}}`)
	writeFile(t, filepath.Join(projPath, ".claude", "settings.json"), `{"model":"sonnet"}`)
	s := newTestServer(t, dir)
	r := s.Router()

	// 已知项目 → 200
	req := httptest.NewRequest("GET", "/api/tree?scope=project&path="+projPath, nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("已知项目应 200, got %d: %s", w.Code, w.Body)
	}

	// 未知路径(越权)→ 4xx
	req2 := httptest.NewRequest("GET", "/api/tree?scope=project&path=/etc", nil)
	req2.Host = "127.0.0.1"
	req2.Header.Set("Authorization", "Bearer tok")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code < 400 || w2.Code >= 500 {
		t.Errorf("未知 path 应 4xx, got %d", w2.Code)
	}
}

// TestGetTreeProjectScopeNoCrossProjectLeak 回归:D1 全 agent 发现后,project scope
// 必须只返回选中项目的资产,不能把其他项目的 ScopeProject 资产泄漏成根级 synthetic 节点。
// 修复前:handler 仅按 a.Scope==ScopeProject 过滤,含所有项目资产 → projB 的
// settings.json(source_path 在 <projB>/.claude,根外)被 BuildTree 挂为根级
// synthetic 节点 "settings.json" 出现在 projA 树里。
func TestGetTreeProjectScopeNoCrossProjectLeak(t *testing.T) {
	dir := t.TempDir()
	// 全局 .claude(避免 Discover 时全局目录缺失警告)
	writeFile(t, filepath.Join(dir, ".claude", "settings.json"), `{}`)

	// 两个项目,各自有 .claude/settings.json
	projA := filepath.Join(dir, "projA")
	projB := filepath.Join(dir, "projB")
	writeFile(t, filepath.Join(projA, ".claude", "settings.json"), `{"model":"sonnet"}`)
	writeFile(t, filepath.Join(projB, ".claude", "settings.json"), `{"model":"opus"}`)
	// 在 .claude.json 登记两个项目(键 = 绝对路径)
	writeFile(t, filepath.Join(dir, ".claude.json"), `{"projects":{"`+projA+`":{},"`+projB+`":{}}}`)

	s := newTestServer(t, dir)
	r := s.Router()

	// 先取 /api/assets 拿到两个项目 settings.json 的资产 ID(按 source_path 区分)。
	areq := httptest.NewRequest("GET", "/api/assets", nil)
	areq.Host = "127.0.0.1"
	areq.Header.Set("Authorization", "Bearer tok")
	aw := httptest.NewRecorder()
	r.ServeHTTP(aw, areq)
	if aw.Code != 200 {
		t.Fatalf("GET /api/assets: got %d: %s", aw.Code, aw.Body)
	}
	var inv configengine.Inventory
	json.Unmarshal(aw.Body.Bytes(), &inv)
	var projAID, projBID string
	for _, a := range inv.Assets {
		if a.Type == configengine.AssetSettings && a.Scope == configengine.ScopeProject {
			if strings.HasPrefix(a.SourcePath, projA+string(filepath.Separator)) {
				projAID = a.ID
			} else if strings.HasPrefix(a.SourcePath, projB+string(filepath.Separator)) {
				projBID = a.ID
			}
		}
	}
	if projAID == "" || projBID == "" {
		t.Fatalf("缺项目 settings 资产: projAID=%q projBID=%q", projAID, projBID)
	}

	// 请求 projA 的 project 树。
	req := httptest.NewRequest("GET", "/api/tree?scope=project&path="+projA, nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("GET /api/tree project: got %d: %s", w.Code, w.Body)
	}

	// 递归收集树里所有 asset_ids。
	var root configengine.TreeNode
	json.Unmarshal(w.Body.Bytes(), &root)
	ids := map[string]bool{}
	var walk func(n configengine.TreeNode)
	walk = func(n configengine.TreeNode) {
		for _, id := range n.AssetIDs {
			ids[id] = true
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)

	// projB 的资产不得出现在 projA 树里(跨项目泄漏)。
	if ids[projBID] {
		t.Errorf("projB 资产 %q 泄漏进 projA 树(共 %d 个 asset_ids)", projBID, len(ids))
	}
	// projA 自己的资产必须在(sanity:树仍对选中项目有效)。
	if !ids[projAID] {
		t.Errorf("projA 资产 %q 缺失(树应含选中项目资产)", projAID)
	}
}

// TestGetTreeProjectRootMissingMcpOnly 回归:项目在 ~/.claude.json 登记,但磁盘上只有
// 根级 .mcp.json 而无 .claude/ 子目录(discoverProjects 允许的场景)。此前 BuildTree 因
// root(<p>/.claude)不存在返回 os.ErrNotExist → handler 500 tree_failed「file does not
// exist」,前端点该标签即报错。修复:project root 缺失时降级 BuildTreeFromAssets,返回
// 只含资产的树(200),保证 .mcp.json 资产仍可见。
func TestGetTreeProjectRootMissingMcpOnly(t *testing.T) {
	dir := t.TempDir()
	// 全局 .claude(避免 Discover 全局目录缺失)
	writeFile(t, filepath.Join(dir, ".claude", "settings.json"), `{}`)

	// 项目只有根级 .mcp.json,无 .claude/ 子目录
	projPath := filepath.Join(dir, "mcpOnly")
	writeFile(t, filepath.Join(dir, ".claude.json"), `{"projects":{"`+projPath+`":{}}}`)
	writeFile(t, filepath.Join(projPath, ".mcp.json"), `{"mcpServers":{"s1":{"command":"x"}}}`)

	s := newTestServer(t, dir)
	r := s.Router()

	req := httptest.NewRequest("GET", "/api/tree?scope=project&path="+projPath, nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("项目 root 缺失应降级返回 200, got %d: %s", w.Code, w.Body)
	}
	var root configengine.TreeNode
	json.Unmarshal(w.Body.Bytes(), &root)
	// 根级 .mcp.json 资产应挂在树里(无论作为 file 还是 synthetic 节点)
	var ids []string
	var walk func(n configengine.TreeNode)
	walk = func(n configengine.TreeNode) {
		ids = append(ids, n.AssetIDs...)
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	if len(ids) == 0 {
		t.Errorf("项目 root 缺失时应仍展示资产,实际 asset_ids 为空")
	}
}
