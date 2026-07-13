package security

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
)

func TestSecretDetectorUnavailable(t *testing.T) {
	d := NewSecretDetector(&config.DetectorsConfig{Secret: config.BinaryDetectorConfig{Enabled: true, Binary: "definitely-not-a-real-binary-xyz"}})
	if d.Available() {
		t.Error("不存在的二进制应 unavailable")
	}
	if d.Reason() == "" {
		t.Error("应有 reason")
	}
	// 不可用时 Scan 不应报错,返回空
	findings, err := d.Scan(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if findings != nil {
		t.Errorf("不可用应无 findings")
	}
}

func TestSecretDetectorParsesGitleaksJSON(t *testing.T) {
	// 用 echo 伪造 gitleaks,输出固定 JSON
	dir := t.TempDir()
	script := filepath.Join(dir, "fakegitleaks")
	os.WriteFile(script, []byte("#!/bin/sh\ncat <<'EOF'\n[{\"RuleID\":\"generic-api-key\",\"Secret\":\"sk-xxx\",\"File\":\"a\",\"StartLine\":1}]\nEOF\n"), 0o755)
	d := NewSecretDetector(&config.DetectorsConfig{Secret: config.BinaryDetectorConfig{Enabled: true, Binary: script}})
	if !d.Available() {
		t.Fatal("fake 应可用")
	}
	a := configengine.Asset{ID: "a1", Type: configengine.AssetMemory, Name: "CLAUDE.md", SourcePath: "a"}
	findings, err := d.Scan(context.Background(), []configengine.Asset{a})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].RuleID != "generic-api-key" {
		t.Fatalf("got %+v", findings)
	}
}

func TestSecretDetectorAttributionByFullPath(t *testing.T) {
	base := t.TempDir()
	// 两个同名文件在不同子目录:top/config.yaml 与 sub/config.yaml
	topFile := filepath.Join(base, "top", "config.yaml")
	subFile := filepath.Join(base, "sub", "config.yaml")
	os.MkdirAll(filepath.Dir(topFile), 0o755)
	os.MkdirAll(filepath.Dir(subFile), 0o755)
	os.WriteFile(topFile, []byte("x"), 0o644)
	os.WriteFile(subFile, []byte("x"), 0o644)

	// 子测试 1:gitleaks 报告 File 为 "sub/config.yaml"(相对 --source)。
	// 资产是 top/config.yaml:basename 都是 config.yaml,但完整路径不同 → 不应归因。
	// 旧 basename 匹配会误归因,新完整路径匹配不会。
	t.Run("no basename mis-attribution", func(t *testing.T) {
		script := filepath.Join(base, "fake_no_match")
		os.WriteFile(script, []byte("#!/bin/sh\necho '[{\"RuleID\":\"k\",\"Secret\":\"sk-leak\",\"File\":\"sub/config.yaml\",\"StartLine\":1}]'\n"), 0o755)
		d := NewSecretDetector(&config.DetectorsConfig{Secret: config.BinaryDetectorConfig{Enabled: true, Binary: script}})
		a := configengine.Asset{ID: "a1", Type: configengine.AssetMemory, Name: "config.yaml", SourcePath: topFile}
		findings, err := d.Scan(context.Background(), []configengine.Asset{a})
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings (full path mismatch), got %d: %+v", len(findings), findings)
		}
	})

	// 子测试 2:gitleaks 报告 File 为 "config.yaml"(相对 --source,即资产本身)。
	// 资产是 sub/config.yaml:完整路径一致 → 应归因。
	t.Run("match by full path", func(t *testing.T) {
		script := filepath.Join(base, "fake_match")
		os.WriteFile(script, []byte("#!/bin/sh\necho '[{\"RuleID\":\"k\",\"Secret\":\"sk-leak\",\"File\":\"config.yaml\",\"StartLine\":1}]'\n"), 0o755)
		d := NewSecretDetector(&config.DetectorsConfig{Secret: config.BinaryDetectorConfig{Enabled: true, Binary: script}})
		a := configengine.Asset{ID: "a2", Type: configengine.AssetMemory, Name: "config.yaml", SourcePath: subFile}
		findings, err := d.Scan(context.Background(), []configengine.Asset{a})
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding (full path match), got %d: %+v", len(findings), findings)
		}
	})
}

func TestSecretDetectorScannerErrorSurfaced(t *testing.T) {
	dir := t.TempDir()
	// fake gitleaks:写 stderr 并以退出码 2 退出(模拟 gitleaks 配置错误)
	script := filepath.Join(dir, "fake_err")
	os.WriteFile(script, []byte("#!/bin/sh\necho 'config error: bad source' >&2\nexit 2\n"), 0o755)
	srcFile := filepath.Join(dir, "config.yaml")
	os.WriteFile(srcFile, []byte("x"), 0o644)
	d := NewSecretDetector(&config.DetectorsConfig{Secret: config.BinaryDetectorConfig{Enabled: true, Binary: script}})
	a := configengine.Asset{ID: "a1", Type: configengine.AssetMemory, Name: "config.yaml", SourcePath: srcFile}
	findings, err := d.Scan(context.Background(), []configengine.Asset{a})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 scanner-error finding, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Severity != SeverityLow {
		t.Errorf("expected SeverityLow, got %s", f.Severity)
	}
	if f.RuleID != "secret.scanner-error" {
		t.Errorf("expected RuleID secret.scanner-error, got %s", f.RuleID)
	}
	if !strings.Contains(f.Evidence, "config error") {
		t.Errorf("expected evidence to contain stderr text, got %q", f.Evidence)
	}
}

// TestSecretDetectorAttributionFirstWinsSharedSourcePath 验证 I-CORR-2:
// parseSettings 对同一 settings.json 产出 settings + permissions + N hook
// 共享 SourcePath。密钥归因应给首个(settings = 文件属主),而非最后一个 hook
// (last-wins 旧逻辑:任意且误导)。first-wins 与依赖检测器的"首个资产"归因一致,
// 且避免按派生视图数(settings/permissions/hooks)重复发 finding 抬高分数。
func TestSecretDetectorAttributionFirstWinsSharedSourcePath(t *testing.T) {
	dir := t.TempDir()
	settingsFile := filepath.Join(dir, "settings.json")
	os.WriteFile(settingsFile, []byte("x"), 0o644)
	// fake gitleaks:报告该 settings.json 含密钥(File 用 basename)。
	script := filepath.Join(dir, "fake_gl")
	os.WriteFile(script, []byte("#!/bin/sh\necho '[{\"RuleID\":\"k\",\"Secret\":\"sk\",\"File\":\"settings.json\",\"StartLine\":1}]'\n"), 0o755)
	d := NewSecretDetector(&config.DetectorsConfig{Secret: config.BinaryDetectorConfig{Enabled: true, Binary: script}})

	// 模拟 parseSettings 的产出顺序:settings 在前,hook 在后,共享 SourcePath。
	settings := configengine.Asset{ID: "settings-id", Type: configengine.AssetSettings, Name: "settings", SourcePath: settingsFile}
	perm := configengine.Asset{ID: "perm-id", Type: configengine.AssetPermissions, Name: "permissions", SourcePath: settingsFile}
	hook := configengine.Asset{ID: "hook-id", Type: configengine.AssetHook, Name: "PreToolUse/*", SourcePath: settingsFile}
	findings, err := d.Scan(context.Background(), []configengine.Asset{settings, perm, hook})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("期望 1 条 finding(同 SourcePath 去重),实际 %d: %+v", len(findings), findings)
	}
	if findings[0].AssetID != "settings-id" {
		t.Errorf("密钥应归因 settings 资产(文件属主),实际 AssetID=%s", findings[0].AssetID)
	}
}
