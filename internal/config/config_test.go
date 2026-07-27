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
		t.Errorf("Language 默认应空(前端回退 en),got %q", cfg.Language)
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

// Task 2:Codex agent 空 root_dir/claude_json → 应落到 ~/.codex 且 ClaudeJSON 为空
// (configengine.KnownAgents() spec: codex 无机器管理文件)。
func TestResolveAgentsCodexDefaultRootDir(t *testing.T) {
	dir := t.TempDir()
	// codex agent 不填 root_dir/claude_json,应落到 ~/.codex 且 claude_json 为空
	cfg := &Config{Agents: []AgentCfg{{ID: "codex", Enabled: true}}}
	agents := cfg.ResolveAgents(dir)
	if len(agents) != 1 || agents[0].ID != "codex" {
		t.Fatalf("got %v", agents)
	}
	wantRoot := filepath.Join(dir, ".codex")
	if agents[0].RootDir != wantRoot {
		t.Fatalf("codex RootDir = %q, want %q", agents[0].RootDir, wantRoot)
	}
	if agents[0].ClaudeJSON != "" {
		t.Fatalf("codex ClaudeJSON = %q, want 空", agents[0].ClaudeJSON)
	}
}

// Task 2:Claude agent 空 root_dir/claude_json → 默认 ~/.claude + ~/.claude.json
func TestResolveAgentsClaudeDefaultPaths(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{Agents: []AgentCfg{{ID: "claude-code", Enabled: true}}}
	agents := cfg.ResolveAgents(dir)
	if agents[0].RootDir != filepath.Join(dir, ".claude") {
		t.Fatalf("claude RootDir = %q", agents[0].RootDir)
	}
	if agents[0].ClaudeJSON != filepath.Join(dir, ".claude.json") {
		t.Fatalf("claude ClaudeJSON = %q", agents[0].ClaudeJSON)
	}
}

// Task 2:未知 agent ID 应回退 Claude(向后兼容旧配置/拼写),防 spec 复用退化。
func TestResolveAgentsUnknownIDFallsBackToClaude(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{Agents: []AgentCfg{{ID: "unknown-agent", Enabled: true}}}
	agents := cfg.ResolveAgents(dir)
	if agents[0].RootDir != filepath.Join(dir, ".claude") || agents[0].ClaudeJSON != filepath.Join(dir, ".claude.json") {
		t.Fatalf("未知 ID 应回退 Claude, got %+v", agents[0])
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

// Task 1:AgentCfg.ScanEnabled 三态 + ResolveScanAgents
func TestScanEnabledTriState(t *testing.T) {
	// nil → true(旧配置向后兼容)
	var nilPtr *bool
	a := AgentCfg{ID: "test", Enabled: true, ScanEnabled: nilPtr}
	if !a.ScanEnabledEffective() {
		t.Error("nil ScanEnabled 应视为 true")
	}
	// 显式 false
	f := false
	a.ScanEnabled = &f
	if a.ScanEnabledEffective() {
		t.Error("false ScanEnabled 应视为 false")
	}
	// 显式 true
	tr := true
	a.ScanEnabled = &tr
	if !a.ScanEnabledEffective() {
		t.Error("true ScanEnabled 应视为 true")
	}
}

func TestResolveScanAgents_FiltersDisabled(t *testing.T) {
	home := t.TempDir()
	tr, f := true, false
	c := &Config{
		Agents: []AgentCfg{
			{ID: "a", Enabled: true, ScanEnabled: &tr},
			{ID: "b", Enabled: true, ScanEnabled: &f},  // 扫描关闭
			{ID: "c", Enabled: false, ScanEnabled: &tr}, // setup 未启用
		},
	}
	got := c.ResolveScanAgents(home)
	if len(got) != 1 || got[0].ID != "a" {
		t.Errorf("应只返回 a: got %v", got)
	}
}

func TestResolveScanAgents_NilMeansAll(t *testing.T) {
	home := t.TempDir()
	c := &Config{
		Agents: []AgentCfg{
			{ID: "x", Enabled: true}, // ScanEnabled nil → true
			{ID: "y", Enabled: true},
		},
	}
	got := c.ResolveScanAgents(home)
	if len(got) != 2 {
		t.Errorf("nil ScanEnabled 应全保留: got %d", len(got))
	}
}

// Task 17:Token 字段往返(service install 写入,后台进程读取)。
func TestTokenFieldRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	cfg := &Config{Token: "abc123", Language: "en"}
	if err := Save(p, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Token != "abc123" {
		t.Errorf("Token 往返: got %q want abc123", got.Token)
	}
}

// Task 3:KnownProjects 独立项目清单(去重,按 Path 保留首个)。
func TestResolveKnownProjectsDedup(t *testing.T) {
	cfg := &Config{KnownProjects: []KnownProject{
		{Path: "/a", Name: "a"},
		{Path: "/a", Name: "dup"}, // 同 path 去重,保留首个
		{Path: "/b", Name: "b"},
	}}
	got := cfg.ResolveKnownProjects()
	if len(got) != 2 || got[0].Path != "/a" || got[1].Path != "/b" {
		t.Fatalf("ResolveKnownProjects = %+v", got)
	}
}

func TestResolveKnownProjectsEmpty(t *testing.T) {
	cfg := &Config{}
	if got := cfg.ResolveKnownProjects(); len(got) != 0 {
		t.Fatalf("空 KnownProjects 应返回空切片, got %+v", got)
	}
}
