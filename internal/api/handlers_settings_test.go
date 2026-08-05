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
	// 注入 UserPrefsStore 预置 language
	s.UserPrefs = config.NewUserPrefsStore(s.DB)
	_ = s.UserPrefs.Set("language", "en")
	s.Config.ClaudeDir = "/custom/.claude"
	s.Config.Discovery = &config.DiscoveryCfg{DisabledAssetTypes: []string{"mcp_server"}}
	w := reqScheduler(t, s, "GET", "/api/settings", nil)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	var st map[string]any
	json.Unmarshal(w.Body.Bytes(), &st)
	if st["language"] != "en" || st["claude_dir"] != "/custom/.claude" {
		t.Errorf("settings: %+v", st)
	}
	// DiscoveryCfg 必须以 snake_case json tag 序列化
	disc, ok := st["discovery"].(map[string]any)
	if !ok {
		t.Fatalf("discovery 缺失或类型错误: %+v", st["discovery"])
	}
	if _, ok := disc["disabled_asset_types"]; !ok {
		t.Errorf("discovery 应含 disabled_asset_types(snake_case json tag),got: %+v", disc)
	}
	if _, ok := disc["DisabledAssetTypes"]; ok {
		t.Errorf("discovery 不应含大写驼峰 DisabledAssetTypes(缺 json tag 的回归),got: %+v", disc)
	}
}

func TestPutSettingsLanguagePersists(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	// 注入 UserPrefsStore
	s.UserPrefs = config.NewUserPrefsStore(s.DB)
	w := reqScheduler(t, s, "PUT", "/api/settings", map[string]any{"language": "en"})
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	// 验证经 UserPrefsStore 持久化
	v, err := s.UserPrefs.Get("language")
	if err != nil || v != "en" {
		t.Errorf("language 未持久化: err=%v, val=%q", err, v)
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

// TestPutSettingsScanToggle 覆盖 scan_enabled 总开关经 PUT /api/settings
// 传播到 ScheduleManager.Paused。
func TestPutSettingsScanToggle(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.SchedRepo = config.NewScheduleRepo(s.DB)
	s.ScheduleManager = scheduler.NewManager(func(string) func(context.Context) error {
		return func(context.Context) error { return nil }
	})
	// 关闭总开关 → Paused=true
	w := reqScheduler(t, s, "PUT", "/api/settings", map[string]any{"scan_enabled": false})
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	if !s.ScheduleManager.Paused() {
		t.Error("scan_enabled=false 应令 ScheduleManager.Paused()=true")
	}
	// 开启 → Paused=false
	w2 := reqScheduler(t, s, "PUT", "/api/settings", map[string]any{"scan_enabled": true})
	if w2.Code != 200 {
		t.Fatalf("got %d: %s", w2.Code, w2.Body)
	}
	if s.ScheduleManager.Paused() {
		t.Error("scan_enabled=true 应令 ScheduleManager.Paused()=false")
	}
}

// TestPutSettingsRejectsBadScanInterval 覆盖 Minor gap:PUT /api/settings 拒绝坏的 scan_interval。
func TestPutSettingsRejectsBadScanInterval(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	w := reqScheduler(t, s, "PUT", "/api/settings", map[string]any{
		"scan_enabled":  true,
		"scan_interval": "not-a-duration",
	})
	if w.Code != 400 {
		t.Fatalf("无效 scan_interval 应 400,got %d: %s", w.Code, w.Body)
	}
}

// TestPutSettingsZeroIntervalPersists 覆盖 Minor:scan_interval="0s" + scan_enabled=true 应 200 OK。
// putSettings 不做 interval<=0 强制禁用(该语义在 /api/scheduler 与 /api/schedules 的 validateInterval 里)。
func TestPutSettingsZeroIntervalPersists(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	s.SchedRepo = config.NewScheduleRepo(s.DB)
	s.ScheduleManager = scheduler.NewManager(func(string) func(context.Context) error {
		return func(context.Context) error { return nil }
	})
	// scan_interval="0s" + scan_enabled=true → putSettings 接受
	w := reqScheduler(t, s, "PUT", "/api/settings", map[string]any{
		"scan_enabled":  true,
		"scan_interval": "0s",
	})
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	// 验证 ScheduleManager.Paused 状态(request scan_enabled=true → Paused=false)
	if s.ScheduleManager.Paused() {
		t.Error("scan_enabled=true 应令 Paused=false,无关 scan_interval 正负")
	}
	// 验证 ScheduleRepo 持久化
	interval, enabled := s.schedPrefs("claude-code")
	if !enabled || interval != "0s" {
		t.Errorf("SchedRepo: enabled=%v interval=%q", enabled, interval)
	}
}
