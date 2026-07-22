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
	// DiscoveryCfg 必须以 snake_case json tag 序列化(与 PinnedProject 同类约束,防 gin
	// 默认大写驼峰 DisabledAssetTypes 污染 /api/settings 响应)。
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

// TestPutSettingsScanToggle 覆盖 scan_enabled 总开关经 PUT /api/settings
// 传播到 ScheduleManager.Paused(而非 dead s.Scheduler.Reconfigure)。
// 旧 TestPutSettingsScanReconfigure 断言 s.Scheduler.Status(),但 putSettings
// 的 scanChanged 分支已改为调 applyScanToggle(只动 ScheduleManager.Paused,
// 不再触 s.Scheduler.Reconfigure),旧测试的断言路径已消失——故以本测试取代之。
// s.Scheduler 字段本身及其余依赖它的测试(TestPutSettingsZeroIntervalDisables 等)
// 由 Task 3 统一清理。
func TestPutSettingsScanToggle(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
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
	if !s.Config.ScanEnabled {
		t.Error("config.ScanEnabled 应已更新为 true")
	}
}

// TestPutSettingsRejectsBadScanInterval 覆盖 Minor gap (d):PUT /api/settings
// 拒绝坏的 scan_interval(此前只测了 /api/scheduler 的坏间隔)。
func TestPutSettingsRejectsBadScanInterval(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	s.Scheduler = scheduler.New(0, func(context.Context) error { return nil })
	w := reqScheduler(t, s, "PUT", "/api/settings", map[string]any{
		"scan_enabled":  true,
		"scan_interval": "not-a-duration",
	})
	if w.Code != 400 {
		t.Fatalf("无效 scan_interval 应 400,got %d: %s", w.Code, w.Body)
	}
}

// TestPutSettingsZeroIntervalDisables 覆盖 Minor #2/#3:scan_interval <= 0 时
// 即使 scan_enabled=true,Reconfigure 也应等价关闭(interval<=0 = 关,Task 7 语义)。
// 验证 putSettings 与 putScheduler 行为一致:零/负 interval 强制 enabled=false。
func TestPutSettingsZeroIntervalDisables(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	s.Scheduler = scheduler.New(0, func(context.Context) error { return nil })
	// scan_interval="0s" + scan_enabled=true → interval<=0 守卫应强制 disabled
	w := reqScheduler(t, s, "PUT", "/api/settings", map[string]any{
		"scan_enabled":  true,
		"scan_interval": "0s",
	})
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	st := s.Scheduler.Status()
	if st.Enabled {
		t.Error("interval<=0 应强制 disabled,但 scheduler 仍 enabled")
	}
}
