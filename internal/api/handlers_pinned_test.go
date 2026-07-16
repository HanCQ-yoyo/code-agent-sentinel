package api

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/config"
)

func TestPinnedProjectsGetEmpty(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	w := reqScheduler(t, s, "GET", "/api/pinned-projects", nil)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	arr, _ := resp["pinned_projects"].([]any)
	if len(arr) != 0 {
		t.Errorf("空应 [] ,got %v", arr)
	}
}

func TestPinnedProjectsPutPersists(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	body := map[string]any{"pinned_projects": []map[string]any{
		{"path": "/proj/a", "color": "red"},
		{"path": "/proj/b", "color": "blue"},
	}}
	w := reqScheduler(t, s, "PUT", "/api/pinned-projects", body)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	if len(s.Config.PinnedProjects) != 2 {
		t.Errorf("内存应 2 项,got %d", len(s.Config.PinnedProjects))
	}
	cfg, _ := config.Load(s.ConfigPath)
	if len(cfg.PinnedProjects) != 2 || cfg.PinnedProjects[0].Path != "/proj/a" {
		t.Errorf("落盘错误: %+v", cfg.PinnedProjects)
	}
}
