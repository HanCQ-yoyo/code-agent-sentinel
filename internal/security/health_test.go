package security

import (
	"math"
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
	for range 50 {
		findings = append(findings, Finding{AssetID: "a", AssetType: configengine.AssetMCPServer, Severity: SeverityCritical, RuleID: "r"})
	}
	h := ComputeHealth(assets, findings)
	if h.Score != 0 {
		t.Errorf("全满应 0, got %d", h.Score)
	}
}

// assertReducible 验证可还原不变式:Σ Deduction.Points ≈ 100 − Score。
// Score 是 int(score+0.5) 四舍五入来的,与浮点 ΣPoints 之间天然有 ≤0.5 的
// 舍入差,故容差取 0.6(0.5 边界 + 余量)。核心诉求:扣分不因封顶而虚高
// (旧实现 pre-cap 算法会让 ΣPoints 远超 100−Score,本测试即捕获该缺陷)。
func assertReducible(t *testing.T, h *HealthScore) {
	t.Helper()
	var sum float64
	for _, d := range h.Deductions {
		sum += d.Points
	}
	want := 100.0 - float64(h.Score)
	if math.Abs(sum-want) > 0.6 {
		t.Errorf("可还原失败: ΣPoints=%.4f, 100-Score=%.4f (Score=%d)", sum, want, h.Score)
	}
}

// TestHealthReducibilityMixed 校验混合严重度场景下扣分可还原。
func TestHealthReducibilityMixed(t *testing.T) {
	assets := []configengine.Asset{
		{ID: "a", Type: configengine.AssetMCPServer, Name: "m"},
		{ID: "b", Type: configengine.AssetSkill, Name: "s"},
	}
	findings := []Finding{
		{AssetID: "a", AssetType: configengine.AssetMCPServer, Severity: SeverityCritical, RuleID: "r1"},
		{AssetID: "a", AssetType: configengine.AssetMCPServer, Severity: SeverityHigh, RuleID: "r2"},
		{AssetID: "b", AssetType: configengine.AssetSkill, Severity: SeverityMedium, RuleID: "r3"},
		{AssetID: "b", AssetType: configengine.AssetSkill, Severity: SeverityLow, RuleID: "r4"},
	}
	h := ComputeHealth(assets, findings)
	assertReducible(t, h)
}

// TestHealthReducibilityCappedAsset 校验 I-CORR-4:单资产风险超 Rmax 被封顶后,
// 扣分仍可还原。5 个 High(原始 12.5,封顶到 10)——旧实现用 pre-cap p 算 Points,
// ΣPoints 会 > 100−Score,破坏可还原。
func TestHealthReducibilityCappedAsset(t *testing.T) {
	assets := []configengine.Asset{
		{ID: "a", Type: configengine.AssetMCPServer, Name: "m"},
		{ID: "b", Type: configengine.AssetSkill, Name: "s"},
	}
	var findings []Finding
	for range 5 {
		findings = append(findings, Finding{AssetID: "a", AssetType: configengine.AssetMCPServer, Severity: SeverityHigh, RuleID: "r"})
	}
	// b 上挂一条 Low,确保多资产场景下分配无误。
	findings = append(findings, Finding{AssetID: "b", AssetType: configengine.AssetSkill, Severity: SeverityLow, RuleID: "r5"})
	h := ComputeHealth(assets, findings)
	// 确认 a 确实被封顶(原始 12.5 > Rmax=10)。
	rawA := 5 * severityCoeff[SeverityHigh]
	if rawA <= Rmax {
		t.Fatalf("测试前提不成立:资产 a 原始风险 %.2f 未超 Rmax", rawA)
	}
	assertReducible(t, h)
}
