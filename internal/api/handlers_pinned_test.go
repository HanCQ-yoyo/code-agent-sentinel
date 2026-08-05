package api

import (
	"encoding/json"
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
	// 注入 UserPrefsStore 使置顶项目可持久化
	s.UserPrefs = config.NewUserPrefsStore(s.DB)
	body := map[string]any{"pinned_projects": []map[string]any{
		{"path": "/proj/a", "color": "red"},
		{"path": "/proj/b", "color": "blue"},
	}}
	w := reqScheduler(t, s, "PUT", "/api/pinned-projects", body)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	// 验证经 UserPrefsStore 持久化
	v, err := s.UserPrefs.Get("pinned_projects")
	if err != nil || v == "" {
		t.Fatalf("pinned_projects 未持久化到 UserPrefsStore: err=%v, val=%q", err, v)
	}
	var projs []config.PinnedProject
	json.Unmarshal([]byte(v), &projs)
	if len(projs) != 2 || projs[0].Path != "/proj/a" {
		t.Errorf("落盘错误: %+v", projs)
	}
}
