package config

import (
	"path/filepath"
	"testing"
)

// nil-safe:未配置(nil)→ 全启用、默认二进制。
func TestDetectorsConfigNilSafe(t *testing.T) {
	var c *DetectorsConfig
	if !c.RulesEnabled() {
		t.Error("nil RulesEnabled 应 true")
	}
	if !c.SecretEnabled() {
		t.Error("nil SecretEnabled 应 true")
	}
	if got := c.SecretBinaryOrDefault(); got != "gitleaks" {
		t.Errorf("nil SecretBinaryOrDefault = %q, want gitleaks", got)
	}
	if !c.DepEnabled() {
		t.Error("nil DepEnabled 应 true")
	}
	if !c.DepEngineEnabled("npm") {
		t.Error("nil DepEngineEnabled(npm) 应 true")
	}
	if got := c.DepEngineBinaryOrDefault("npm"); got != "npm" {
		t.Errorf("nil DepEngineBinaryOrDefault(npm) = %q, want npm", got)
	}
	if got := c.DepEngineBinaryOrDefault("govulncheck"); got != "govulncheck" {
		t.Errorf("nil DepEngineBinaryOrDefault(govulncheck) = %q, want govulncheck", got)
	}
}

// 显式禁用 + 自定义二进制:访问器反映配置。
func TestDetectorsConfigExplicit(t *testing.T) {
	c := &DetectorsConfig{
		Rules:  DetectorToggle{Enabled: false},
		Secret: BinaryDetectorConfig{Enabled: true, Binary: "/usr/local/bin/gitleaks"},
		Dep: DepDetectorConfig{
			Enabled: true,
			Engines: map[string]BinaryDetectorConfig{
				"npm":         {Enabled: false, Binary: ""},
				"govulncheck": {Enabled: true, Binary: "/go/bin/govulncheck"},
			},
		},
	}
	if c.RulesEnabled() {
		t.Error("RulesEnabled 应 false")
	}
	if !c.SecretEnabled() {
		t.Error("SecretEnabled 应 true")
	}
	if got := c.SecretBinaryOrDefault(); got != "/usr/local/bin/gitleaks" {
		t.Errorf("SecretBinaryOrDefault = %q", got)
	}
	if c.DepEngineEnabled("npm") {
		t.Error("npm 应禁用")
	}
	if !c.DepEngineEnabled("govulncheck") {
		t.Error("govulncheck 应启用")
	}
	if got := c.DepEngineBinaryOrDefault("govulncheck"); got != "/go/bin/govulncheck" {
		t.Errorf("govulncheck binary = %q", got)
	}
	// 空 binary → 默认名
	c.Secret.Binary = ""
	if got := c.SecretBinaryOrDefault(); got != "gitleaks" {
		t.Errorf("空 binary 应回退默认, got %q", got)
	}
}

// EnsureDetectors:nil → 分配全启用默认;已存在 → 不覆盖。
func TestEnsureDetectors(t *testing.T) {
	c := DefaultConfig()
	c.EnsureDetectors()
	if c.Detectors == nil {
		t.Fatal("EnsureDetectors 后 Detectors 仍 nil")
	}
	if !c.Detectors.SecretEnabled() {
		t.Error("默认应全启用")
	}
	// 已存在配置不被覆盖
	c.Detectors.Rules = DetectorToggle{Enabled: false}
	c.EnsureDetectors()
	if c.Detectors.RulesEnabled() {
		t.Error("已存在的配置应保留(Rules 仍禁用)")
	}
}

// YAML 往返:序列化不含 mutex 字段;反序列化恢复配置。
func TestDetectorsConfigYAMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	c := DefaultConfig()
	c.EnsureDetectors()
	c.Detectors.Secret.Binary = "/opt/gitleaks"
	c.Detectors.Dep.Engines = map[string]BinaryDetectorConfig{"npm": {Enabled: false}}
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Detectors.SecretEnabled() {
		t.Error("加载后 Secret 应启用")
	}
	if got := loaded.Detectors.SecretBinaryOrDefault(); got != "/opt/gitleaks" {
		t.Errorf("加载后 Secret binary = %q", got)
	}
	if loaded.Detectors.DepEngineEnabled("npm") {
		t.Error("加载后 npm 应禁用")
	}
}
