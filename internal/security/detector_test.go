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
