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
// s.Scheduler 字段本身由 Task 3 连同 handlers_scheduler.go 的 dead 分支删除。
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
	w := reqScheduler(t, s, "PUT", "/api/settings", map[string]any{
		"scan_enabled":  true,
		"scan_interval": "not-a-duration",
	})
	if w.Code != 400 {
		t.Fatalf("无效 scan_interval 应 400,got %d: %s", w.Code, w.Body)
	}
}

// TestPutSettingsZeroIntervalPersists 覆盖 Minor #2/#3:scan_interval <= 0 时
// 即使 scan_enabled=true,putSettings 也不应崩溃且 config 应如实落盘。
//
// Task 3 重构注记:旧版本断言 s.Scheduler.Status().Enabled 被强制为 false——
// 那依赖已删除的 s.Scheduler.Reconfigure(interval<=0 → enabled=false)路径。
// 现 putSettings 不再做 interval<=0 → 强制禁用 的等价处理(该语义只在
// /api/scheduler 与 /api/schedules 的 validateInterval 里;scan_interval 是
// /api/settings 总开关级别,仅作 ResolveSchedules 回退默认)。applyScanToggle
// 只传播 ScanEnabled → ScheduleManager.Paused,与 scan_interval 正负无关。
//
// 故本测试改为验证真实行为:scan_interval="0s" + scan_enabled=true → 200 OK,
// config 落盘如实(ScanEnabled=true / ScanInterval="0s"),ScheduleManager.Paused=false
// (总开关开),且不 panic。对 interval<=0 强制禁用的语义覆盖由 /api/scheduler
// 与 /api/schedules 的测试(TestPutSchedulerEnablesAndPersists 零间隔分支、
// TestPostScheduleRejectsBadInterval)承担。
func TestPutSettingsZeroIntervalPersists(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	s.ScheduleManager = scheduler.NewManager(func(string) func(context.Context) error {
		return func(context.Context) error { return nil }
	})
	// scan_interval="0s" + scan_enabled=true → putSettings 接受(0s 是合法 duration),
	// 总开关开 → ScheduleManager.Paused=false,config 如实落盘。
	w := reqScheduler(t, s, "PUT", "/api/settings", map[string]any{
		"scan_enabled":  true,
		"scan_interval": "0s",
	})
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	if !s.Config.ScanEnabled {
		t.Error("ScanEnabled 应为 true(如实落盘)")
	}
	if s.Config.ScanInterval != "0s" {
		t.Errorf("ScanInterval 应为 \"0s\",got %q", s.Config.ScanInterval)
	}
	if s.ScheduleManager.Paused() {
		t.Error("scan_enabled=true 应令 Paused=false,无关 scan_interval 正负")
	}
	// 落盘校验
	cfg, _ := config.Load(s.ConfigPath)
	if !cfg.ScanEnabled || cfg.ScanInterval != "0s" {
		t.Errorf("文件未如实落盘: enabled=%v interval=%q", cfg.ScanEnabled, cfg.ScanInterval)
	}
}
