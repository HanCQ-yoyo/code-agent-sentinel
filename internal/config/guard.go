package config

import "sync"

// GuardConfig 是运行时拦截(guard)的配置段,与 Detectors 平级。
// 持 sync.RWMutex:server 侧 PUT /api/guard/config 写(Lock)与 guard 子进程读并发安全。
// hook 子进程不共享 server 内存,每次 config.Load 读盘;server 写盘→guard 读盘。
//
// nil-safe:所有访问器在接收者为 nil 时返回"全启用 + 默认值",兼容旧配置(无 guard 段)
// 与测试(传 nil)。main.go 启动时调 Config.EnsureGuard() 确保非 nil,使 PUT 原地改写
// 能被即时看到(不可整体替换指针)。
type GuardConfig struct {
	mu              sync.RWMutex
	Enabled         bool   `yaml:"enabled" json:"enabled"`                     // 总开关
	Policy          string `yaml:"policy" json:"policy"`                       // "deny"(v1 唯一;预留 warn/ask)
	DeadlineMS      int    `yaml:"deadline_ms" json:"deadline_ms"`             // 评估预算 ms,默认 200
	MaxCommandBytes int    `yaml:"max_command_bytes" json:"max_command_bytes"` // 命令长度上限,超限 fail-open
	// Stage R3:安全模式。strict(默认,异常宁可误拦)/lenient(不确定时 ask 替代 deny)。
	// 安全不变量 #1:高置信度命中两模式都 deny,Mode 只影响"不确定"降级。
	Mode string `yaml:"mode" json:"mode"`
	// Stage R3:allowlist 总开关。true(默认)时管线 ⑦ 精确匹配放行;false 跳过放行。
	AllowlistEnabled bool `yaml:"allowlist_enabled" json:"allowlist_enabled"`
}

// EnabledEffective nil → true(向后兼容:无 guard 段视为启用拦截)。
func (c *GuardConfig) EnabledEffective() bool {
	if c == nil {
		return true
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Enabled
}

// PolicyOrDefault nil/空 → "deny"。
func (c *GuardConfig) PolicyOrDefault() string {
	if c == nil {
		return "deny"
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Policy == "" {
		return "deny"
	}
	return c.Policy
}

// DeadlineOrDefault nil/≤0 → 200。
func (c *GuardConfig) DeadlineOrDefault() int {
	if c == nil {
		return 200
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.DeadlineMS <= 0 {
		return 200
	}
	return c.DeadlineMS
}

// MaxBytesOrDefault nil/≤0 → 262144(256KB)。
func (c *GuardConfig) MaxBytesOrDefault() int {
	if c == nil {
		return 262144
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.MaxCommandBytes <= 0 {
		return 262144
	}
	return c.MaxCommandBytes
}

// ModeOrDefault nil/空 → "strict"(向后兼容:无 mode 段视为严格)。
// 安全不变量:Mode 只影响"不确定"降级(strict→deny/lenient→ask);高置信度命中两模式都 deny。
func (c *GuardConfig) ModeOrDefault() string {
	if c == nil {
		return "strict"
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Mode == "" {
		return "strict"
	}
	return c.Mode
}

// AllowlistEnabledOrDefault nil → true(向后兼容:默认启用放行)。
func (c *GuardConfig) AllowlistEnabledOrDefault() bool {
	if c == nil {
		return true
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.AllowlistEnabled
}

// ApplyFrom 在写锁下把 other 的字段(不含 mu)复制到 c(原地改写,不替换指针)。
// 仿 DetectorsConfig.ApplyFrom:server 侧 PUT 用,检测器/guard 持指针即时生效。
func (c *GuardConfig) ApplyFrom(other *GuardConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()
	c.Enabled = other.Enabled
	c.Policy = other.Policy
	c.DeadlineMS = other.DeadlineMS
	c.MaxCommandBytes = other.MaxCommandBytes
	c.Mode = other.Mode
	c.AllowlistEnabled = other.AllowlistEnabled
}
