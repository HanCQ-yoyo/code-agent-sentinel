package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGuardConfigNilSafe(t *testing.T) {
	var c *GuardConfig // nil
	if !c.EnabledEffective() {
		t.Fatal("nil GuardConfig 应 EnabledEffective=true(向后兼容)")
	}
	if got := c.PolicyOrDefault(); got != "deny" {
		t.Fatalf("nil PolicyOrDefault=%q, want deny", got)
	}
	if got := c.DeadlineOrDefault(); got != 200 {
		t.Fatalf("nil DeadlineOrDefault=%d, want 200", got)
	}
	if got := c.MaxBytesOrDefault(); got != 262144 {
		t.Fatalf("nil MaxBytesOrDefault=%d, want 262144", got)
	}
}

func TestGuardConfigApplyFrom(t *testing.T) {
	dst := &GuardConfig{}
	src := &GuardConfig{Enabled: false, Policy: "warn", DeadlineMS: 500, MaxCommandBytes: 1024}
	dst.ApplyFrom(src)
	if dst.Enabled != false || dst.Policy != "warn" || dst.DeadlineMS != 500 || dst.MaxCommandBytes != 1024 {
		t.Fatalf("ApplyFrom 未复制字段: %+v", dst)
	}
}

func TestEnsureGuard(t *testing.T) {
	cfg := &Config{}
	cfg.EnsureGuard()
	if cfg.Guard == nil {
		t.Fatal("EnsureGuard 未分配 Guard")
	}
	if !cfg.Guard.Enabled || cfg.Guard.Policy != "deny" || cfg.Guard.DeadlineMS != 200 {
		t.Fatalf("EnsureGuard 默认值不对: %+v", cfg.Guard)
	}
	// 已存在不覆盖
	cfg.Guard.Policy = "warn"
	cfg.EnsureGuard()
	if cfg.Guard.Policy != "warn" {
		t.Fatal("EnsureGuard 不应覆盖已存在的 Guard")
	}
}

func TestConfigGuardYAML(t *testing.T) {
	// 验证 Config.Guard 走 YAML 又走 JSON(防 gin 大写驼峰,见 config-struct-json-tag memory)
	data := []byte("guard:\n  enabled: false\n  policy: warn\n  deadline_ms: 300\n  max_command_bytes: 2048\n")
	cfg, err := LoadFromBytes(data) // 见 Step 4 辅助函数
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Guard == nil || cfg.Guard.Enabled != false || cfg.Guard.Policy != "warn" || cfg.Guard.DeadlineMS != 300 || cfg.Guard.MaxCommandBytes != 2048 {
		t.Fatalf("YAML 反序列化 Guard 不对: %+v", cfg.Guard)
	}
}

// LoadFromBytes 从 YAML 字节加载(测试辅助,绕过文件)。
func LoadFromBytes(data []byte) (*Config, error) {
	c := DefaultConfig()
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return c, nil
}
