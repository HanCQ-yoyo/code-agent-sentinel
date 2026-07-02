package security

import (
	"testing"

	"code-agent-sentinel/internal/configengine"
)

func TestHealthNoFindingsIs100(t *testing.T) {
	assets := []configengine.Asset{{ID: "a", Type: configengine.AssetMCPServer}}
	h := ComputeHealth(assets, nil)
	if h.Score != 100 {
		t.Fatalf("无 finding 应 100, got %d", h.Score)
	}
}

func TestHealthMonotonicAndReproducible(t *testing.T) {
	assets := []configengine.Asset{
		{ID: "a", Type: configengine.AssetMCPServer, Name: "m"},
		{ID: "b", Type: configengine.AssetSkill, Name: "s"},
	}
	all := []Finding{
		{AssetID: "a", AssetType: configengine.AssetMCPServer, Severity: SeverityCritical, RuleID: "r1"},
		{AssetID: "a", AssetType: configengine.AssetMCPServer, Severity: SeverityHigh, RuleID: "r2"},
		{AssetID: "b", AssetType: configengine.AssetSkill, Severity: SeverityMedium, RuleID: "r3"},
	}
	h1 := ComputeHealth(assets, all)
	h2 := ComputeHealth(assets, all)
	if h1.Score != h2.Score {
		t.Fatal("不可还原")
	}
	// 去掉一个 finding,分数应升高
	fewer := all[:2]
	hf := ComputeHealth(assets, fewer)
	if hf.Score <= h1.Score {
		t.Errorf("修掉 finding 分数应升: %d -> %d", h1.Score, hf.Score)
	}
	if h1.Score < 0 || h1.Score > 100 {
		t.Errorf("分数越界: %d", h1.Score)
	}
}

func TestHealthAllMaxIsZero(t *testing.T) {
	assets := []configengine.Asset{{ID: "a", Type: configengine.AssetMCPServer, Name: "m"}}
	// 灌大量 critical 直到封顶 Rmax
	var findings []Finding
	for i := 0; i < 50; i++ {
		findings = append(findings, Finding{AssetID: "a", AssetType: configengine.AssetMCPServer, Severity: SeverityCritical, RuleID: "r"})
	}
	h := ComputeHealth(assets, findings)
	if h.Score != 0 {
		t.Errorf("全满应 0, got %d", h.Score)
	}
}
