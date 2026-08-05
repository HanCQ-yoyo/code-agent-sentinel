package config

import (
	"os"
	"path/filepath"

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
	BackupDir  string   `yaml:"backup_dir"`  // 空=默认 ~/.code-agent-sentinel/backups
	MaxBackups int      `yaml:"max_backups"` // 0=默认 20

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
	// Token 是服务模式预置的访问 token。空=前台交互场景随机生成(见 main.go)。
	// service install 写入,使后台进程无需经 banner 展示即可拼接 #token= URL。
	Token string `yaml:"token"`
	// Task 14:日志文件路径。空=stderr(默认,前台交互场景)。
	// --log-path flag > config.LogPath > 默认 stderr,run() 里统一解析。
	// service install 生成的单元文件带 --log-path 指向 <home>/.code-agent-sentinel/code-agent-sentinel.log,
	// 亦允许用户在此显式配置自定义路径覆盖单元默认。
	LogPath string `yaml:"log_path" json:"log_path"`
	// 已知项目清单(独立于 agent 机器文件;setup 可从 ~/.claude.json 导入初始值)。
	// 供 Codex 项目级发现使用(替代 ~/.claude.json projects),Claude 各项目读 .claude/ 亦可用。
	KnownProjects []KnownProject `yaml:"known_projects" json:"known_projects"`
	// 多 agent 配置(setup 写入)。空 → ResolveAgents 回退到 ClaudeDir。
	Agents []AgentCfg `yaml:"agents" json:"agents"`
}

// DiscoveryCfg 控制资产发现范围(按资产类型开关)。configengine 不导入本包,
// 故此处用 []string(12 个 AssetType 之一),main.go 桥接为 configengine.AssetType。
type DiscoveryCfg struct {
	DisabledAssetTypes []string `yaml:"disabled_asset_types" json:"disabled_asset_types"`
}

// PinnedProject 是 Assets 页置顶的项目(右键置顶 + 颜色标识)。
// yaml tag 已移除:置顶项目迁移到 SQLite user_prefs 表,不再从 config.yaml 序列化。
type PinnedProject struct {
	Path  string `json:"path"`
	Color string `json:"color"`
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
	RootDir     string `yaml:"root_dir"      json:"root_dir"`              // 配置根:~/.claude;空=默认
	ClaudeJSON  string `yaml:"claude_json"   json:"claude_json"`           // 机器管理文件:~/.claude.json;空=默认
}

// ScheduleCfg 是单个 agent 的定时扫描任务配置。
// yaml tag 已移除:调度配置迁移到 SQLite schedules 表,不再从 config.yaml 序列化。
type ScheduleCfg struct {
	AgentID  string `json:"agent_id"` // "claude-code"
	Enabled  bool   `json:"enabled"`
	Interval string `json:"interval"` // "30m"/"1h";空/0/无效=关
}

func DefaultConfig() *Config {
	return &Config{Bind: "127.0.0.1", Port: 15921, MaxBackups: 20}
}

// DefaultPath 返回 ~/.code-agent-sentinel/config.yaml。
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".code-agent-sentinel", "config.yaml"), nil
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

// resolveDefault 空串返回 def,否则返回 v。供 ResolveAgents 填默认路径用。
func resolveDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
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

// ResolveScanAgents 返回所有 Enabled 的 agent 子集(不再按 ScanEnabled 过滤,该职责迁移到 ScheduleRepo)。
func (c *Config) ResolveScanAgents(home string) []AgentCfg {
	all := c.ResolveAgents(home)
	out := make([]AgentCfg, 0, len(all))
	for _, a := range all {
		if a.Enabled {
			out = append(out, a)
		}
	}
	return out
}

// ResolveSchedules 返回 nil(调度配置从 SQLite ScheduleRepo 读取,不再从 config.yaml 反序列化)。
func (c *Config) ResolveSchedules(agents []AgentCfg) []ScheduleCfg {
	return nil
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
