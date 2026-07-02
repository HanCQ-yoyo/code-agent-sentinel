package security

import (
	"context"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

func TestOrchestratorScan(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeDetector{id: "fake", avail: true})
	r.Register(fakeDetector{id: "off", avail: false})
	o := &Orchestrator{Registry: r}
	assets := []configengine.Asset{{ID: "x", Type: configengine.AssetHook}}
	res, err := o.Scan(context.Background(), assets, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 {
		t.Errorf("findings: %d", len(res.Findings))
	}
	// off 不可用:不出 finding,但 status 记录 unavailable
	offOK := false
	for _, s := range res.Detectors {
		if s.ID == "off" && !s.Available {
			offOK = true
		}
	}
	if !offOK {
		t.Error("off 检测器应标记 unavailable")
	}
}

func TestOrchestratorSelectiveDetectors(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeDetector{id: "a", avail: true})
	r.Register(fakeDetector{id: "b", avail: true})
	o := &Orchestrator{Registry: r}
	res, _ := o.Scan(context.Background(), nil, []string{"a"})
	if len(res.Detectors) != 1 || res.Detectors[0].ID != "a" {
		t.Errorf("应只跑 a: %+v", res.Detectors)
	}
}
