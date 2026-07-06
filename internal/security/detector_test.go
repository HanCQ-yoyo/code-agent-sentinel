package security

import (
	"context"
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
	cases := []struct {
		name        string
		d           Detector
		wantName    string
		wantEngines int
		wantRules   int
		wantCovers  int
	}{
		{"baseline", NewBaselineDetector(), "基线检测", 1, 4, 2},
		{"injection", NewInjectionDetector(), "提示注入检测", 1, 3, 6},
		{"secret", NewSecretDetector(""), "密钥检测", 1, 0, 0},
		{"dep", NewDependencyDetector("", ""), "依赖检测", 2, 0, 4},
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
		})
	}
}
