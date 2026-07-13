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

func TestDependencyUnavailableWhenNoNPM(t *testing.T) {
	d := NewDependencyDetector(&config.DetectorsConfig{Dep: config.DepDetectorConfig{
		Enabled: true,
		Engines: map[string]config.BinaryDetectorConfig{
			"npm":         {Enabled: true, Binary: "no-npm-xyz"},
			"govulncheck": {Enabled: true, Binary: "no-govulncheck-xyz"},
		},
	}})
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
	d := NewDependencyDetector(&config.DetectorsConfig{Dep: config.DepDetectorConfig{
		Enabled: true,
		Engines: map[string]config.BinaryDetectorConfig{
			"npm":         {Enabled: true, Binary: script},
			"govulncheck": {Enabled: true, Binary: "no-govulncheck-xyz"},
		},
	}})
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
	d := NewDependencyDetector(&config.DetectorsConfig{Dep: config.DepDetectorConfig{
		Enabled: true,
		Engines: map[string]config.BinaryDetectorConfig{
			"npm":         {Enabled: true, Binary: script},
			"govulncheck": {Enabled: true, Binary: "no-govulncheck-xyz"},
		},
	}})
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
	d := NewDependencyDetector(&config.DetectorsConfig{Dep: config.DepDetectorConfig{
		Enabled: true,
		Engines: map[string]config.BinaryDetectorConfig{
			"npm":         {Enabled: true, Binary: "no-npm-xyz"},
			"govulncheck": {Enabled: true, Binary: script},
		},
	}})
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

// TestDependencyGovulncheckParsesFindings 验证 C-CORR-1:parseGovulncheck 能从
// 真实的 govulncheck -json NDJSON 输出解析出漏洞 finding。
//
// 真实 govulncheck 输出含 4 类记录:{"config":...}、{"osv":{...object...}}、
// {"finding":{"osv":"GO-...",...}}、{"progress":...}。NONE 有顶层 string osv。
// 旧解析器期望顶层 {"osv":"<string>","severity":"<string>"},对所有真实记录
// 解析出 0 个 finding → govulncheck 后端是死代码。新解析器按 {"finding":{"osv":...}}
// 记录产 finding。
func TestDependencyGovulncheckParsesFindings(t *testing.T) {
	// 仿真 govulncheck -json NDJSON:config + osv 对象 + 2 条 finding + progress。
	ndjson := `{"config":{"db":"vulndb"}}
{"osv":{"id":"GO-2024-0001","severity":[{"type":"CVSS_V3","score":"high"}],"summary":"rce in foo"}}
{"osv":{"id":"GO-2024-0002","severity":[{"type":"CVSS_V3","score":"medium"}],"summary":"dos in bar"}}
{"finding":{"osv":"GO-2024-0001","trace":[{"module":"foo","version":"v1.0.0"}]}}
{"finding":{"osv":"GO-2024-0002","trace":[{"module":"bar","version":"v2.0.0"}]}}
{"progress":{"last-scanned-module":"baz"}}
`
	a := configengine.Asset{ID: "g1", Type: configengine.AssetPlugin, Name: "pkg"}
	findings := parseGovulncheck("dep", []byte(ndjson), a)
	if len(findings) != 2 {
		t.Fatalf("应解析出 2 个 finding(每条 finding 记录一个),得到 %d: %+v", len(findings), findings)
	}
	// 校验两条 finding 的 OSV ID、RuleID、Message、AssetID。
	seen := map[string]bool{}
	for _, f := range findings {
		if f.AssetID != "g1" {
			t.Errorf("AssetID 应为 g1,得到 %s", f.AssetID)
		}
		if f.AssetType != configengine.AssetPlugin {
			t.Errorf("AssetType 应为 plugin,得到 %s", f.AssetType)
		}
		osvID := strings.TrimPrefix(f.RuleID, "dep.govulncheck.")
		if osvID != "GO-2024-0001" && osvID != "GO-2024-0002" {
			t.Fatalf("意外 OSV ID %s (RuleID=%s)", osvID, f.RuleID)
		}
		if seen[osvID] {
			t.Errorf("OSV ID %s 重复", osvID)
		}
		seen[osvID] = true
		if !strings.Contains(f.Message, osvID) {
			t.Errorf("Message 应包含 OSV ID %s,得到 %s", osvID, f.Message)
		}
		if !strings.Contains(f.Remediation, osvID) {
			t.Errorf("Remediation 应包含 OSV ID %s,得到 %s", osvID, f.Remediation)
		}
	}
	for _, id := range []string{"GO-2024-0001", "GO-2024-0002"} {
		if !seen[id] {
			t.Errorf("缺少 OSV ID %s 的 finding", id)
		}
	}
}
