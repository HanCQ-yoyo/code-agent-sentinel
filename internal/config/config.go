package config

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"code-agent-sentinel/internal/configengine"
)

type BasicAuth struct {
	User         string `yaml:"user"`
	PasswordHash string `yaml:"password_hash"` // bcrypt
}

type Config struct {
	Bind         string     `yaml:"bind"`
	Port         int        `yaml:"port"`
	AllowedCIDRs []string   `yaml:"allowed_cidrs"`
	BasicAuth    *BasicAuth `yaml:"basic_auth"`
	HomeDir      string     `yaml:"home_dir"` // 覆盖 ~/.claude 的 home
	// DirTags 用户对「目录标签」的显式覆盖:key=相对 .claude 根的路径,value=标签。
	// 默认标签见 DefaultDirTags();生效标签由 ResolveDirTag 合并。
	// 空表示用户未自定义,全用默认。见 dir_tags.go。
	DirTags    DirTags  `yaml:"dir_tags"`
	Favorites  []string `yaml:"favorites"`   // 资产收藏/置顶 id 列表(跨会话保留;localStorage 受端口影响故改存配置)
	BackupDir  string   `yaml:"backup_dir"`  // 空=默认 ~/.claude-sentinel/backups
	MaxBackups int      `yaml:"max_backups"` // 0=默认 20

	// Task 15:安全检测增强配置字段。空值=用默认路径/值,Resolve* 方法统一解析。
	SentinelRulesDir    string  `yaml:"sentinel_rules_dir"`   // 空=默认 ~/.claude-sentinel/rules
	SuppressPath        string  `yaml:"suppress_path"`        // 空=默认 ~/.claude-sentinel/suppressions.yaml
	BaselinePath        string  `yaml:"baseline_path"`        // 空=默认 ~/.claude-sentinel/baseline.json
	SuppressionDiscount float64 `yaml:"suppression_discount"` // 空/0=默认 0.3

	// 检测器运行期配置(启用开关 + 二进制路径)。nil=全启用默认(向后兼容)。
	// main.go 启动时 EnsureDetectors 确保非 nil,使 API 写能原地被检测器读到。
	Detectors *DetectorsConfig `yaml:"detectors"`

	// 运行时拦截(guard)配置。nil=全启用默认(向后兼容,无 guard 段)。
	// main.go 启动时 EnsureGuard 确保非 nil,使 PUT /api/guard/config 原地改写生效。
	Guard *GuardConfig `yaml:"guard" json:"guard"`

	// #2:.claude 目录绝对路径;空 = home/.claude
	ClaudeDir string `yaml:"claude_dir"`
	// #2:发现范围开关;nil = 全发现
	Discovery *DiscoveryCfg `yaml:"discovery"`
	// #1:定时扫描间隔(如 "30m"/"1h");空/0/无效 = 关
	ScanInterval string `yaml:"scan_interval"`
	// #1:定时扫描总开关
	ScanEnabled bool `yaml:"scan_enabled"`
	// #5:"zh"/"en";空 = 前端默认英文(用户可用 localStorage 覆盖,或在此显式配置)
	Language string `yaml:"language"`
	// Token 是服务模式预置的访问 token。空=前台交互场景随机生成(见 main.go)。
	// service install 写入,使后台进程无需经 banner 展示即可拼接 #token= URL。
	Token string `yaml:"token"`
	// Task 14:日志文件路径。空=stderr(默认,前台交互场景)。
	// --log-path flag > config.LogPath > 默认 stderr,run() 里统一解析。
	// service install 生成的单元文件带 --log-path 指向 <home>/.claude-sentinel/sentinel.log,
	// 亦允许用户在此显式配置自定义路径覆盖单元默认。
	LogPath string `yaml:"log_path" json:"log_path"`
	// #4:置顶项目列表
	PinnedProjects []PinnedProject `yaml:"pinned_projects"`
	// 已知项目清单(独立于 agent 机器文件;setup 可从 ~/.claude.json 导入初始值)。
	// 供 Codex 项目级发现使用(替代 ~/.claude.json projects),Claude 各项目读 .claude/ 亦可用。
	// 与 PinnedProjects 并列(语义不同:已知清单 vs Assets 页置顶标识)。
	KnownProjects []KnownProject `yaml:"known_projects" json:"known_projects"`
	// 多 agent 配置(setup 写入)。空 → ResolveAgents 回退到 ClaudeDir。
	Agents []AgentCfg `yaml:"agents" json:"agents"`
	// 多任务调度:每个 agent 一个定时扫描任务。空 → ResolveSchedules 回退到 ScanEnabled/ScanInterval。
	Schedules []ScheduleCfg `yaml:"schedules" json:"schedules"`
}

// DiscoveryCfg 控制资产发现范围(按资产类型开关)。configengine 不导入本包,
// 故此处用 []string(12 个 AssetType 之一),main.go 桥接为 configengine.AssetType。
type DiscoveryCfg struct {
	DisabledAssetTypes []string `yaml:"disabled_asset_types" json:"disabled_asset_types"`
}

// PinnedProject 是 Assets 页置顶的项目(右键置顶 + 颜色标识)。
type PinnedProject struct {
	Path  string `yaml:"path" json:"path"`
	Color string `yaml:"color" json:"color"`
}

// KnownProject 是 sentinel 独立维护的已知项目清单(不依赖任何 agent 的机器文件)。
// 用于两个 agent 的项目级发现:Claude 读各项目 .claude/,Codex 读各项目 AGENTS.md/.codex/。
// 与 PinnedProjects 并列(语义不同:已知清单 vs Assets 页置顶标识)。
type KnownProject struct {
	Path string `yaml:"path" json:"path"`
	Name string `yaml:"name" json:"name"`
}

// ResolveKnownProjects 返回去重后的已知项目列表(按 Path 去重,空 Path 跳过,保留首次出现)。
func (c *Config) ResolveKnownProjects() []KnownProject {
	seen := map[string]bool{}
	out := make([]KnownProject, 0, len(c.KnownProjects))
	for _, p := range c.KnownProjects {
		if p.Path == "" || seen[p.Path] {
			continue
		}
		seen[p.Path] = true
		out = append(out, p)
	}
	return out
}

// AgentCfg 是单个 code agent 的用户配置(setup 写入)。
type AgentCfg struct {
	ID          string `yaml:"id"            json:"id"`
	Enabled     bool   `yaml:"enabled"       json:"enabled"`               // setup 勾选结果
	ScanEnabled *bool  `yaml:"scan_enabled,omitempty" json:"scan_enabled"` // 运行期扫描覆盖开关。nil→默认 true(向后兼容旧配置)
	RootDir     string `yaml:"root_dir"      json:"root_dir"`              // 配置根:~/.claude;空=默认
	ClaudeJSON  string `yaml:"claude_json"   json:"claude_json"`           // 机器管理文件:~/.claude.json;空=默认
}

// ScanEnabledEffective 展开 ScanEnabled 三态:nil→true(向后兼容旧配置)。
func (a *AgentCfg) ScanEnabledEffective() bool {
	if a.ScanEnabled == nil {
		return true
	}
	return *a.ScanEnabled
}

// ScheduleCfg 是单个 agent 的定时扫描任务配置。
type ScheduleCfg struct {
	AgentID  string `yaml:"agent_id" json:"agent_id"` // "claude-code"
	Enabled  bool   `yaml:"enabled"  json:"enabled"`
	Interval string `yaml:"interval" json:"interval"` // "30m"/"1h";空/0/无效=关
}

func DefaultConfig() *Config {
	return &Config{Bind: "127.0.0.1", Port: 15921, MaxBackups: 20}
}

// DefaultPath 返回 ~/.claude-sentinel/config.yaml。
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude-sentinel", "config.yaml"), nil
}

// Load 从 path 加载配置;文件不存在返回默认。
func Load(path string) (*Config, error) {
	c := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Save 将配置写回 path(覆盖写)。目录不存在则创建(0o700:含可能的凭据)。
// 用于 /api/dir-tags 等运行期回写用户偏好。
func Save(path string, c *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// DefaultSuppressionDiscount 是抑制 finding 的残值扣分因子(决策 #12:残值 30% 扣分)。
// SuppressionDiscount 为 0 或负值时用此默认。
const DefaultSuppressionDiscount = 0.3

// ResolveSentinelRulesDir 返回全局规则目录路径。空=默认 <home>/.claude-sentinel/rules。
func (c *Config) ResolveSentinelRulesDir(home string) string {
	if c.SentinelRulesDir != "" {
		return c.SentinelRulesDir
	}
	return filepath.Join(home, ".claude-sentinel", "rules")
}

// ResolveClaudeDir 解析 .claude 目录绝对路径:非空用配置值,空回退 home/.claude。
func (c *Config) ResolveClaudeDir(home string) string {
	if c.ClaudeDir != "" {
		return c.ClaudeDir
	}
	return filepath.Join(home, ".claude")
}

// ResolveAgents 解析启用的 agent 列表。
// Agents 非空 → 直用(逐项空字段按 ID 查 configengine.KnownAgents() spec 填默认);为空 → 用旧 ClaudeDir 回退构造单项 claude-code。
// 保证旧配置(claude_dir)零破坏。codex 的 spec 默认 ClaudeJSON="" → Codex agent 不再借 ~/.claude.json。
func (c *Config) ResolveAgents(home string) []AgentCfg {
	if len(c.Agents) > 0 {
		specs := map[string]configengine.AgentSpec{}
		for _, s := range configengine.KnownAgents() {
			specs[s.ID] = s
		}
		out := make([]AgentCfg, len(c.Agents))
		for i, a := range c.Agents {
			spec, ok := specs[a.ID]
			if !ok {
				// 未知 agent ID:回退当 claude-code(向后兼容旧配置/拼写)。
				spec = specs["claude-code"]
			}
			a.RootDir = resolveDefault(a.RootDir, spec.DefaultRootDir(home))
			a.ClaudeJSON = resolveDefault(a.ClaudeJSON, spec.DefaultClaudeJSON(home))
			out[i] = a
		}
		return out
	}
	// 回退:用 ClaudeDir(可能空 → 默认 home/.claude)构造单项。
	return []AgentCfg{{
		ID:         "claude-code",
		Enabled:    true,
		RootDir:    c.ResolveClaudeDir(home),
		ClaudeJSON: filepath.Join(home, ".claude.json"),
	}}
}

// ResolveScanAgents 返回「Enabled(加载)且 ScanEnabledEffective(扫描)」的 agent 子集。
func (c *Config) ResolveScanAgents(home string) []AgentCfg {
	all := c.ResolveAgents(home)
	out := make([]AgentCfg, 0, len(all))
	for _, a := range all {
		if a.Enabled && a.ScanEnabledEffective() {
			out = append(out, a)
		}
	}
	return out
}

// ResolveSchedules 解析定时任务列表。
// Schedules 非空 → 直用;为空且旧 ScanEnabled+ScanInterval 有效 → 回退造首 agent 单任务。
func (c *Config) ResolveSchedules(agents []AgentCfg) []ScheduleCfg {
	if len(c.Schedules) > 0 {
		return c.Schedules
	}
	if !c.ScanEnabled || c.ScanInterval == "" {
		return nil
	}
	if d, err := time.ParseDuration(c.ScanInterval); err != nil || d <= 0 {
		return nil
	}
	firstAgent := "claude-code"
	if len(agents) > 0 {
		firstAgent = agents[0].ID
	}
	return []ScheduleCfg{{AgentID: firstAgent, Enabled: true, Interval: c.ScanInterval}}
}

// resolveDefault 空串返回 def,否则返回 v。供 ResolveAgents 填默认路径用。
func resolveDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// ResolveSuppressPath 返回 suppressions 文件路径。空=默认 <home>/.claude-sentinel/suppressions.yaml。
func (c *Config) ResolveSuppressPath(home string) string {
	if c.SuppressPath != "" {
		return c.SuppressPath
	}
	return filepath.Join(home, ".claude-sentinel", "suppressions.yaml")
}

// ResolveBaselinePath 返回 baseline 文件路径。空=默认 <home>/.claude-sentinel/baseline.json。
func (c *Config) ResolveBaselinePath(home string) string {
	if c.BaselinePath != "" {
		return c.BaselinePath
	}
	return filepath.Join(home, ".claude-sentinel", "baseline.json")
}

// ResolveStatesPath 返回 finding_states.yaml 路径。统一默认 <home>/.claude-sentinel/finding_states.yaml。
// 注:finding_states.yaml 暂不接 config 覆盖(与 baseline/suppressions 不同),统一默认路径
// 简化迁移与多路径一致性问题。
func (c *Config) ResolveStatesPath(home string) string {
	return filepath.Join(home, ".claude-sentinel", "finding_states.yaml")
}

// ResolveSuppressionDiscount 返回抑制折扣因子。0 或负值=默认 0.3。
func (c *Config) ResolveSuppressionDiscount() float64 {
	if c.SuppressionDiscount > 0 {
		return c.SuppressionDiscount
	}
	return DefaultSuppressionDiscount
}

// EnsureDetectors 确保 c.Detectors 非 nil(分配全启用默认)。已存在则不覆盖。
// 供 main.go 启动时调用:检测器持有 *DetectorsConfig 指针,PUT /api/detectors/config
// 原地改写其字段,故指针须在构造检测器前就稳定指向一个非 nil 对象。
//
// 注意:&DetectorsConfig{} 的零值是全 false(全禁用),与"全启用默认"语义相反,
// 故需显式设 Enabled=true。bool 零值是 false,无法区分"未设"与"显式禁用",
// 但 nil-safe 访问器已覆盖"无 detectors 段"的情况(nil→全启用),此处覆盖"新建"的情况。
//
// YAML 契约:若手写 detectors: 段,必须指定全部三个检测器(rules/secret/dep)。
// 部分段(如只写 rules:)会因 bool 零值=false 静默禁用未指定的检测器。
// 纯 bool 无法在反序列化后区分"键缺失"与"显式 false",故 Load 路径不做自动修复;
// PUT /api/detectors/config 端点在 API 层做了顶层键齐全校验(见 putDetectorConfig),
// 手编 YAML 由用户负责写完整。
func (c *Config) EnsureDetectors() {
	if c.Detectors == nil {
		c.Detectors = &DetectorsConfig{
			Rules:  DetectorToggle{Enabled: true},
			Secret: BinaryDetectorConfig{Enabled: true},
			Dep:    DepDetectorConfig{Enabled: true},
		}
	}
}

// EnsureGuard 确保 c.Guard 非 nil(分配全启用默认)。已存在则不覆盖。
// 仿 EnsureDetectors:guard 子进程 / API 持指针,须在构造前稳定指向非 nil 对象。
func (c *Config) EnsureGuard() {
	if c.Guard == nil {
		c.Guard = &GuardConfig{Enabled: true, Policy: "deny", DeadlineMS: 200, Mode: "strict", AllowlistEnabled: true}
	}
}
