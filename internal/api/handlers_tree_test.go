package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
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
