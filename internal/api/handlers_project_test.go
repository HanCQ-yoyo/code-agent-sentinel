package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

func TestGetProjectNoCurrent(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	r := s.Router()
	req := httptest.NewRequest("GET", "/api/project", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	var body map[string]json.RawMessage
	json.Unmarshal(w.Body.Bytes(), &body)
	if _, ok := body["current"]; ok {
		t.Error("getProject 不应再返回 current(单项目选择已移除)")
	}
	if _, ok := body["projects"]; !ok {
		t.Error("getProject 应返回 projects 列表")
	}
}

// TestGetProjectFiltersHomePseudoProject 回归:~/.claude.json 把 home 本身登记为
// 项目时(path == home),其 .claude 就是全局配置根,与全局 tab 完全重复。
// getProject 必须把它从项目列表里剔除,避免资产 tab 出现"全局 + home"两个相同视图。
func TestGetProjectFiltersHomePseudoProject(t *testing.T) {
	dir := t.TempDir()
	// 全局根存在(home/.claude),否则 DefaultAgents 的 RootDir 指向空目录(过滤仍成立,
	// 但更接近真实)。
	writeFile(t, filepath.Join(dir, ".claude", "settings.json"), `{}`)
	// .claude.json 登记两个项目:home 本身(伪) + 一个真实子项目。
	projReal := filepath.Join(dir, "realproj")
	writeFile(t, filepath.Join(projReal, ".claude", "settings.json"), `{}`)
	writeFile(t, filepath.Join(dir, ".claude.json"), `{"projects":{"`+dir+`":{},"`+projReal+`":{}}}`)

	s := newTestServer(t, dir)
	r := s.Router()
	req := httptest.NewRequest("GET", "/api/project", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	var body struct {
		Projects []configengine.Project `json:"projects"`
	}
	json.Unmarshal(w.Body.Bytes(), &body)
	for _, p := range body.Projects {
		if filepath.Clean(p.Path) == filepath.Clean(dir) {
			t.Errorf("home 本身(伪项目 %s)不应出现在项目列表里, got %v", dir, body.Projects)
		}
	}
	// 真实项目应保留。
	found := false
	for _, p := range body.Projects {
		if filepath.Clean(p.Path) == filepath.Clean(projReal) {
			found = true
		}
	}
	if !found {
		t.Errorf("真实项目 %s 应保留, got %v", projReal, body.Projects)
	}
}

func TestPostProjectRemoved(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	r := s.Router()
	req := httptest.NewRequest("POST", "/api/project?path=/tmp/foo", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("POST /api/project 应已移除(404),实际 %d", w.Code)
	}
}
