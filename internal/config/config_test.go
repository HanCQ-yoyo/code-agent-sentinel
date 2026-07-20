package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadDefaultsWhenMissing(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Bind != "127.0.0.1" {
		t.Errorf("默认 bind: %s", c.Bind)
	}
	if c.Port != 0 {
		t.Errorf("默认 port 应 0(随机): %d", c.Port)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte("bind: 0.0.0.0\nport: 8080\nallowed_cidrs: [\"10.0.0.0/8\"]\nbasic_auth:\n  user: admin\n  password_hash: \"$2a$\"\n"), 0o644)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Bind != "0.0.0.0" || c.Port != 8080 {
		t.Errorf("解析错: %+v", c)
	}
	if len(c.AllowedCIDRs) != 1 || c.AllowedCIDRs[0] != "10.0.0.0/8" {
		t.Errorf("cidrs: %v", c.AllowedCIDRs)
	}
	if c.BasicAuth == nil || c.BasicAuth.User != "admin" {
		t.Errorf("basic auth: %+v", c.BasicAuth)
	}
}

func TestConfigHasNoProjectField(t *testing.T) {
	// --project 启动项已下线,Config 不应再有 Project 字段(若残留则断言失败)。
	var c Config
	_, ok := reflect.TypeOf(c).FieldByName("Project")
	if ok {
		t.Fatal("Config 不应再有 Project 字段(--project 已移除)")
	}
}

func TestConfigBackupDefaults(t *testing.T) {
	c := DefaultConfig()
	if c.MaxBackups != 20 {
		t.Fatalf("default MaxBackups want 20 got %d", c.MaxBackups)
	}
	if c.BackupDir != "" {
		t.Fatalf("default BackupDir want empty(resolved at editor.New) got %q", c.BackupDir)
	}
}

// Task 15:新字段默认值 + Resolve* helpers
func TestConfigResolveDefaults(t *testing.T) {
	c := DefaultConfig()
	home := "/tmp/fake-home"

	if got := c.ResolveSentinelRulesDir(home); got != filepath.Join(home, ".claude-sentinel", "rules") {
		t.Errorf("ResolveSentinelRulesDir = %q", got)
	}
	if got := c.ResolveSuppressPath(home); got != filepath.Join(home, ".claude-sentinel", "suppressions.yaml") {
		t.Errorf("ResolveSuppressPath = %q", got)
	}
	if got := c.ResolveBaselinePath(home); got != filepath.Join(home, ".claude-sentinel", "baseline.json") {
		t.Errorf("ResolveBaselinePath = %q", got)
	}
	if got := c.ResolveSuppressionDiscount(); got != DefaultSuppressionDiscount {
		t.Errorf("ResolveSuppressionDiscount = %v, want %v", got, DefaultSuppressionDiscount)
	}
}

func TestConfigResolveOverrides(t *testing.T) {
	c := DefaultConfig()
	c.SentinelRulesDir = "/custom/rules"
	c.SuppressPath = "/custom/suppr.yaml"
	c.BaselinePath = "/custom/baseline.json"
	c.SuppressionDiscount = 0.5

	if got := c.ResolveSentinelRulesDir("/home"); got != "/custom/rules" {
		t.Errorf("ResolveSentinelRulesDir override = %q", got)
	}
	if got := c.ResolveSuppressPath("/home"); got != "/custom/suppr.yaml" {
		t.Errorf("ResolveSuppressPath override = %q", got)
	}
	if got := c.ResolveBaselinePath("/home"); got != "/custom/baseline.json" {
		t.Errorf("ResolveBaselinePath override = %q", got)
	}
	if got := c.ResolveSuppressionDiscount(); got != 0.5 {
		t.Errorf("ResolveSuppressionDiscount override = %v", got)
	}
}

func TestConfigNewFieldsDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ClaudeDir != "" {
		t.Errorf("ClaudeDir 默认应空,got %q", cfg.ClaudeDir)
	}
	if cfg.Discovery != nil {
		t.Error("Discovery 默认应 nil(全发现)")
	}
	if cfg.ScanInterval != "" {
		t.Errorf("ScanInterval 默认应空(关),got %q", cfg.ScanInterval)
	}
	if cfg.ScanEnabled {
		t.Error("ScanEnabled 默认应 false")
	}
	if cfg.Language != "" {
		t.Errorf("Language 默认应空(回退 zh),got %q", cfg.Language)
	}
	if cfg.PinnedProjects != nil {
		t.Error("PinnedProjects 默认应 nil")
	}
}

func TestConfigResolveClaudeDir(t *testing.T) {
	home := "/home/alice"
	// 空 → 默认 home/.claude
	cfg := DefaultConfig()
	if got := cfg.ResolveClaudeDir(home); got != filepath.Join(home, ".claude") {
		t.Errorf("空 claude_dir 应回退 %q,got %q", filepath.Join(home, ".claude"), got)
	}
	// 非空 → 用配置值
	cfg.ClaudeDir = "/custom/.claude"
	if got := cfg.ResolveClaudeDir(home); got != "/custom/.claude" {
		t.Errorf("非空应原样返回,got %q", got)
	}
}

func TestConfigLoadDiscoveryAndPinned(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	write := `claude_dir: /custom/.claude
discovery:
  disabled_asset_types: [skill, command]
scan_interval: 30m
scan_enabled: true
language: en
pinned_projects:
  - path: /proj/a
    color: red
`
	if err := os.WriteFile(path, []byte(write), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ClaudeDir != "/custom/.claude" {
		t.Errorf("ClaudeDir: %q", cfg.ClaudeDir)
	}
	if cfg.Discovery == nil || len(cfg.Discovery.DisabledAssetTypes) != 2 {
		t.Fatalf("Discovery 解析错误: %+v", cfg.Discovery)
	}
	if cfg.Discovery.DisabledAssetTypes[0] != "skill" {
		t.Errorf("DisabledAssetTypes[0]: %q", cfg.Discovery.DisabledAssetTypes[0])
	}
	if cfg.ScanInterval != "30m" {
		t.Errorf("ScanInterval: %q", cfg.ScanInterval)
	}
	if !cfg.ScanEnabled {
		t.Error("ScanEnabled 应 true")
	}
	if cfg.Language != "en" {
		t.Errorf("Language: %q", cfg.Language)
	}
	if len(cfg.PinnedProjects) != 1 || cfg.PinnedProjects[0].Path != "/proj/a" || cfg.PinnedProjects[0].Color != "red" {
		t.Errorf("PinnedProjects: %+v", cfg.PinnedProjects)
	}
}

func TestResolveAgentsUsesAgentsWhenNonEmpty(t *testing.T) {
	home := t.TempDir()
	c := &Config{Agents: []AgentCfg{
		{ID: "claude-code", Enabled: true, RootDir: "/custom/.claude"},
	}}
	got := c.ResolveAgents(home)
	if len(got) != 1 || got[0].ID != "claude-code" || got[0].RootDir != "/custom/.claude" {
		t.Fatalf("ResolveAgents 应直用 agents: %+v", got)
	}
}

func TestResolveAgentsFillsDefaultPaths(t *testing.T) {
	home := t.TempDir()
	c := &Config{Agents: []AgentCfg{{ID: "claude-code", Enabled: true}}}
	got := c.ResolveAgents(home)
	if got[0].RootDir != filepath.Join(home, ".claude") {
		t.Errorf("空 RootDir 应填默认: got %q", got[0].RootDir)
	}
	if got[0].ClaudeJSON != filepath.Join(home, ".claude.json") {
		t.Errorf("空 ClaudeJSON 应填默认: got %q", got[0].ClaudeJSON)
	}
}

func TestResolveAgentsFallsBackToClaudeDir(t *testing.T) {
	home := t.TempDir()
	c := &Config{ClaudeDir: "/old/.claude"} // Agents 空
	got := c.ResolveAgents(home)
	if len(got) != 1 || got[0].ID != "claude-code" || !got[0].Enabled {
		t.Fatalf("Agents 空应回退 claude_dir 构造单 agent: %+v", got)
	}
	if got[0].RootDir != "/old/.claude" {
		t.Errorf("回退 RootDir 应=claude_dir: got %q", got[0].RootDir)
	}
}

func TestResolveAgentsFallsBackToDefaultWhenAllEmpty(t *testing.T) {
	home := t.TempDir()
	c := &Config{} // Agents 与 ClaudeDir 都空
	got := c.ResolveAgents(home)
	if len(got) != 1 || got[0].RootDir != filepath.Join(home, ".claude") {
		t.Fatalf("全空应回退默认 home/.claude: %+v", got)
	}
}

func TestResolveSchedulesUsesSchedulesWhenNonEmpty(t *testing.T) {
	c := &Config{Schedules: []ScheduleCfg{{AgentID: "claude-code", Enabled: true, Interval: "30m"}}}
	got := c.ResolveSchedules(nil)
	if len(got) != 1 || got[0].AgentID != "claude-code" || got[0].Interval != "30m" {
		t.Fatalf("ResolveSchedules 应直用 schedules: %+v", got)
	}
}

func TestResolveSchedulesFallsBackToScanEnabled(t *testing.T) {
	c := &Config{ScanEnabled: true, ScanInterval: "1h"}
	agents := []AgentCfg{{ID: "claude-code", Enabled: true}}
	got := c.ResolveSchedules(agents)
	if len(got) != 1 || got[0].AgentID != "claude-code" || got[0].Interval != "1h" || !got[0].Enabled {
		t.Fatalf("Schedules 空应回退 scan_* 造首 agent 任务: %+v", got)
	}
}

func TestResolveSchedulesEmptyWhenScanDisabled(t *testing.T) {
	c := &Config{ScanEnabled: false, ScanInterval: ""}
	got := c.ResolveSchedules([]AgentCfg{{ID: "claude-code", Enabled: true}})
	if len(got) != 0 {
		t.Fatalf("scan 关闭且回退时应返回空: %+v", got)
	}
}
