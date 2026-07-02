package security

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

func TestDependencyUnavailableWhenNoNPM(t *testing.T) {
	d := NewDependencyDetector("no-npm-xyz", "no-govulncheck-xyz")
	if d.Available() {
		t.Error("应 unavailable")
	}
	findings, err := d.Scan(context.Background(), nil)
	if err != nil || findings != nil {
		t.Errorf("不可用应空且无错: %v %v", findings, err)
	}
}

func TestDependencyParsesNpmAudit(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "pkg"), 0o755)
	pkgDir := filepath.Join(dir, "pkg")
	os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{"name":"p"}`), 0o644)
	script := filepath.Join(dir, "fakenpm")
	os.WriteFile(script, []byte("#!/bin/sh\ncat <<'EOF'\n{\"vulnerabilities\":{\"lodash\":{\"severity\":\"high\",\"via\":\"x\"}}}\nEOF\n"), 0o755)
	d := NewDependencyDetector(script, "no-govulncheck-xyz")
	if !d.Available() {
		t.Fatal("fake npm 应可用")
	}
	a := configengine.Asset{ID: "p1", Type: configengine.AssetPlugin, Name: "pkg", SourcePath: pkgDir}
	findings, err := d.Scan(context.Background(), []configengine.Asset{a})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) == 0 {
		t.Fatal("未解析出漏洞")
	}
	if findings[0].Severity != SeverityHigh {
		t.Errorf("严重度: %s", findings[0].Severity)
	}
}

func TestDependencyNpmScannerErrorSurfaced(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "pkg")
	os.MkdirAll(pkgDir, 0o755)
	os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{"name":"p"}`), 0o644)
	// fake npm:写 stderr 并以退出码 2 退出(模拟 .npmrc/网络/lock 文件故障)
	script := filepath.Join(dir, "fakenpm")
	os.WriteFile(script, []byte("#!/bin/sh\necho \"npm error: ENOTFOUND registry\" >&2\nexit 2\n"), 0o755)
	d := NewDependencyDetector(script, "no-govulncheck-xyz")
	if !d.Available() {
		t.Fatal("fake npm 应可用")
	}
	a := configengine.Asset{ID: "p1", Type: configengine.AssetPlugin, Name: "pkg", SourcePath: pkgDir}
	findings, err := d.Scan(context.Background(), []configengine.Asset{a})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("期望 1 个扫描器错误 finding,得到 %d: %v", len(findings), findings)
	}
	f := findings[0]
	if f.RuleID != "dep.scanner-error" {
		t.Errorf("RuleID: %s", f.RuleID)
	}
	if f.Severity != SeverityLow {
		t.Errorf("Severity: %s", f.Severity)
	}
	if !strings.Contains(f.Evidence, "npm error") {
		t.Errorf("Evidence 应包含 stderr 文本,得到: %s", f.Evidence)
	}
}

func TestDependencyGovulncheckScannerErrorSurfaced(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "pkg")
	os.MkdirAll(pkgDir, 0o755)
	os.WriteFile(filepath.Join(pkgDir, "go.mod"), []byte("module p\n\ngo 1.22\n"), 0o644)
	// fake govulncheck:写 stderr 并以退出码 2 退出(模拟安装损坏/go.mod 无效/崩溃)
	script := filepath.Join(dir, "fakegovulncheck")
	os.WriteFile(script, []byte("#!/bin/sh\necho \"govulncheck: failed to load package\" >&2\nexit 2\n"), 0o755)
	d := NewDependencyDetector("no-npm-xyz", script)
	if !d.Available() {
		t.Fatal("fake govulncheck 应可用")
	}
	a := configengine.Asset{ID: "g1", Type: configengine.AssetPlugin, Name: "pkg", SourcePath: pkgDir}
	findings, err := d.Scan(context.Background(), []configengine.Asset{a})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("期望 1 个扫描器错误 finding,得到 %d: %v", len(findings), findings)
	}
	f := findings[0]
	if f.RuleID != "dep.scanner-error" {
		t.Errorf("RuleID: %s", f.RuleID)
	}
	if f.Severity != SeverityLow {
		t.Errorf("Severity: %s", f.Severity)
	}
	if !strings.Contains(f.Evidence, "govulncheck") {
		t.Errorf("Evidence 应包含 stderr 文本,得到: %s", f.Evidence)
	}
}
