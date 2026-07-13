package config

import "sync"

// DetectorToggle 是无二进制的检测器启用开关(如 rules)。
type DetectorToggle struct {
	Enabled bool `yaml:"enabled"`
}

// BinaryDetectorConfig 是带二进制路径的检测器/引擎配置(如 secret、dep 的各引擎)。
type BinaryDetectorConfig struct {
	Enabled bool   `yaml:"enabled"`
	Binary  string `yaml:"binary"` // 空=用默认二进制名
}

// DepDetectorConfig 是依赖检测器配置:检测器级开关 + 各引擎(npm/govulncheck)独立配置。
type DepDetectorConfig struct {
	Enabled bool                                `yaml:"enabled"`
	Engines map[string]BinaryDetectorConfig     `yaml:"engines"` // keys: "npm","govulncheck"
}

// DetectorsConfig 汇总三个检测器的运行期配置。持有 sync.RWMutex:检测器读(RLock)
// 与 PUT /api/detectors/config 写(Lock)并发安全。mu 为非导出字段,yaml/json 均跳过。
//
// nil-safe:所有访问器在接收者为 nil 时返回"全启用 + 默认二进制",兼容旧配置(无 detectors 段)
// 与测试(传 nil)。main.go 启动时调 Config.EnsureDetectors() 确保非 nil,使 PUT 原地改写
// 能被持指针的检测器即时看到(不可整体替换指针,否则检测器持有的旧指针看不到更新)。
type DetectorsConfig struct {
	mu     sync.RWMutex
	Rules  DetectorToggle       `yaml:"rules"`
	Secret BinaryDetectorConfig `yaml:"secret"`
	Dep    DepDetectorConfig    `yaml:"dep"`
}

func (c *DetectorsConfig) RulesEnabled() bool {
	if c == nil {
		return true
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Rules.Enabled
}

func (c *DetectorsConfig) SecretEnabled() bool {
	if c == nil {
		return true
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Secret.Enabled
}

// SecretBinaryOrDefault 返回密钥检测器二进制;空则回退 "gitleaks"。
func (c *DetectorsConfig) SecretBinaryOrDefault() string {
	if c == nil {
		return "gitleaks"
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Secret.Binary != "" {
		return c.Secret.Binary
	}
	return "gitleaks"
}

func (c *DetectorsConfig) DepEnabled() bool {
	if c == nil {
		return true
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Dep.Enabled
}

// DepEngineEnabled 返回某引擎(npm/govulncheck)是否启用。未配置该引擎=启用(默认)。
func (c *DetectorsConfig) DepEngineEnabled(name string) bool {
	if c == nil {
		return true
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Dep.Engines == nil {
		return true
	}
	e, ok := c.Dep.Engines[name]
	if !ok {
		return true // 未配置 = 启用默认
	}
	return e.Enabled
}

// DepEngineBinaryOrDefault 返回某引擎二进制;空则回退引擎名(npm/govulncheck)。
func (c *DetectorsConfig) DepEngineBinaryOrDefault(name string) string {
	if c == nil {
		return name
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Dep.Engines == nil {
		return name
	}
	e, ok := c.Dep.Engines[name]
	if !ok || e.Binary == "" {
		return name
	}
	return e.Binary
}

// ApplyFrom 在写锁下把 other 的配置字段(不含 mu)复制到 c(原地改写,不替换指针)。
// 供 PUT /api/detectors/config 持久化前回写到运行期配置。other 的 mu 不复制。
func (c *DetectorsConfig) ApplyFrom(other *DetectorsConfig) {
	if c == nil || other == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Rules = other.Rules
	c.Secret = other.Secret
	c.Dep = other.Dep
}
