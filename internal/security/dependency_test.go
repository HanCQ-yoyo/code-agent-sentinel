package security

import (
	"context"
	"os"
	"path/filepath"
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
