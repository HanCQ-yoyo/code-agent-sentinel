package api

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/scheduler"
)

func TestGetSettings(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	s.Config.Language = "en"
	s.Config.ClaudeDir = "/custom/.claude"
	w := reqScheduler(t, s, "GET", "/api/settings", nil)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	var st map[string]any
	json.Unmarshal(w.Body.Bytes(), &st)
	if st["language"] != "en" || st["claude_dir"] != "/custom/.claude" {
		t.Errorf("settings: %+v", st)
	}
}

func TestPutSettingsLanguagePersists(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	s.Scheduler = scheduler.New(0, func(context.Context) error { return nil })
	w := reqScheduler(t, s, "PUT", "/api/settings", map[string]any{"language": "en"})
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	if s.Config.Language != "en" {
		t.Error("未更新内存")
	}
	cfg, _ := config.Load(s.ConfigPath)
	if cfg.Language != "en" {
		t.Error("未落盘")
	}
}

func TestPutSettingsIgnoresRestartFieldsWithWarning(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	// 试图改 claude_dir(需重启字段)应被忽略 + warning
	w := reqScheduler(t, s, "PUT", "/api/settings", map[string]any{"language": "zh", "claude_dir": "/evil"})
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	warnings, _ := resp["warnings"].([]any)
	if len(warnings) == 0 {
		t.Error("应有 warning 说明 claude_dir 需重启")
	}
	if s.Config.ClaudeDir == "/evil" {
		t.Error("claude_dir 不应被运行期修改")
	}
}
