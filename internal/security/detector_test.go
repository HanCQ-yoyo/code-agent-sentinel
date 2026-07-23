package security

import (
	"context"
	"strings"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

type fakeDetector struct {
	id    string
	avail bool
}

func (f fakeDetector) ID() string { return f.id }
func (f fakeDetector) Covers() []configengine.AssetType {
	return []configengine.AssetType{configengine.AssetHook}
}
func (f fakeDetector) Enabled() bool    { return true }
func (f fakeDetector) Available() bool { return f.avail }
func (f fakeDetector) Reason() string  { return "" }
func (f fakeDetector) Scan(ctx context.Context, assets []configengine.Asset) ([]Finding, error) {
	return []Finding{{DetectorID: f.id, Severity: SeverityHigh, AssetID: "x"}}, nil
}
func (f fakeDetector) Meta() DetectorMeta {
	return DetectorMeta{ID: f.id, Name: f.id, Engines: []EngineInfo{{Name: "fake", Kind: "embedded", Available: f.avail}}}
}

func TestRegistryRegisterAndList(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeDetector{id: "fake", avail: true})
	if len(r.Detectors()) != 1 {
		t.Fatal("未注册")
	}
	d := r.Get("fake")
	if d == nil {
		t.Fatal("Get 失败")
	}
}

func TestDetectorMeta(t *testing.T) {
	tmpHome := t.TempDir() // 空 home:NewRulesDetector 不读真实 ~/.claude-sentinel
	cases := []struct {
		name        string
		d           Detector
		wantName    string
		wantEngines int
		wantRules   int
		wantCovers  int
	}{
		{"rules", NewRulesDetector(tmpHome, nil), "声明式规则引擎", 1, 213, 0},
		{"secret", NewSecretDetector(nil), "密钥检测", 1, 0, 0},
		{"dep", NewDependencyDetector(nil), "依赖检测", 2, 0, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := c.d.Meta()
			if m.ID != c.d.ID() {
				t.Errorf("Meta ID = %q, want %q", m.ID, c.d.ID())
			}
			if m.Name != c.wantName {
				t.Errorf("Name = %q, want %q", m.Name, c.wantName)
			}
			if len(m.Engines) != c.wantEngines {
				t.Errorf("Engines 数 = %d, want %d: %+v", len(m.Engines), c.wantEngines, m.Engines)
			}
			if len(m.Rules) != c.wantRules {
				t.Errorf("Rules 数 = %d, want %d", len(m.Rules), c.wantRules)
			}
			if len(m.Covers) != c.wantCovers {
				t.Errorf("Covers 数 = %d, want %d", len(m.Covers), c.wantCovers)
			}
			// 引擎必须含 kind 字段(embedded/subprocess)
			for _, e := range m.Engines {
				if e.Kind != "embedded" && e.Kind != "subprocess" {
					t.Errorf("引擎 %q kind 非法: %q", e.Name, e.Kind)
				}
			}
			// rules 每条内嵌规则须含 syntax(可读语法或正则);secret/dep rules 为 nil 不校验。
			if len(m.Rules) > 0 {
				for _, r := range m.Rules {
					if r.Syntax == "" {
						t.Errorf("规则 %q syntax 为空", r.ID)
					}
				}
			}
			// rules 每条应含 asset_type / remediation / source_file 等补齐字段
			if len(m.Rules) > 0 {
				r := m.Rules[0]
				if r.AssetType == "" {
					t.Errorf("规则 %q AssetType 为空", r.ID)
				}
				if r.SourceFile == "" {
					t.Errorf("规则 %q SourceFile 为空", r.ID)
				}
				// Paths 可为 nil(无路径过滤),但字段须存在(零值 nil)
				// Metadata 可为空 map
			}
		})
	}
}

func TestRuleSyntaxContent(t *testing.T) {
	// baseline.wildcard-bash 的 syntax 应含 value "Bash(*)"(op=contains 的可读语法含 value)。
	tmpHome := t.TempDir()
	rd := NewRulesDetector(tmpHome, nil)
	m := rd.Meta()
	var got string
	for _, r := range m.Rules {
		if r.ID == "baseline.wildcard-bash" {
			got = r.Syntax
		}
	}
	if !strings.Contains(got, "Bash(*)") {
		t.Fatalf("baseline.wildcard-bash syntax = %q, want 含 Bash(*)", got)
	}
	// injection 规则 syntax 应为 pattern 正则原文(含 / 包裹,非空)。
	var injSyntax string
	for _, r := range m.Rules {
		if r.ID == "injection.hidden-instruction.skill" {
			injSyntax = r.Syntax
		}
	}
	if injSyntax == "" {
		t.Fatalf("injection.hidden-instruction.skill syntax 为空: %+v", m.Rules)
	}
	if !strings.Contains(injSyntax, "/") {
		t.Fatalf("injection syntax 应含 /pattern/ 形式, got %q", injSyntax)
	}
}
