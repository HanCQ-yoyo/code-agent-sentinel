package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
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
	DirTags    DirTags `yaml:"dir_tags"`
	Favorites  []string `yaml:"favorites"` // 资产收藏/置顶 id 列表(跨会话保留;localStorage 受端口影响故改存配置)
	BackupDir  string  `yaml:"backup_dir"`  // 空=默认 ~/.claude-sentinel/backups
	MaxBackups int     `yaml:"max_backups"` // 0=默认 20

	// Task 15:安全检测增强配置字段。空值=用默认路径/值,Resolve* 方法统一解析。
	SentinelRulesDir    string  `yaml:"sentinel_rules_dir"`    // 空=默认 ~/.claude-sentinel/rules
	SuppressPath        string  `yaml:"suppress_path"`         // 空=默认 ~/.claude-sentinel/suppressions.yaml
	BaselinePath        string  `yaml:"baseline_path"`         // 空=默认 ~/.claude-sentinel/baseline.json
	SuppressionDiscount float64 `yaml:"suppression_discount"`  // 空/0=默认 0.3

	// 检测器运行期配置(启用开关 + 二进制路径)。nil=全启用默认(向后兼容)。
	// main.go 启动时 EnsureDetectors 确保非 nil,使 API 写能原地被检测器读到。
	Detectors *DetectorsConfig `yaml:"detectors"`
}

func DefaultConfig() *Config {
	return &Config{Bind: "127.0.0.1", Port: 0, MaxBackups: 20}
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
