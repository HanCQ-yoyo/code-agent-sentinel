package security

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

func TestSecretDetectorUnavailable(t *testing.T) {
	d := NewSecretDetector("definitely-not-a-real-binary-xyz")
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
	d := NewSecretDetector(script)
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
