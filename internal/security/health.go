package security

import (
	"code-agent-sentinel/internal/configengine"
)

const Rmax = 10.0

var typeWeights = map[configengine.AssetType]float64{
	configengine.AssetMCPServer:   3.0,
	configengine.AssetHook:        3.0,
	configengine.AssetPermissions: 2.5,
	configengine.AssetSettings:    2.0,
	configengine.AssetScript:      2.0,
	configengine.AssetSkill:       1.5,
	configengine.AssetCommand:     1.5,
	configengine.AssetAgent:       1.5,
	configengine.AssetPlugin:      1.5,
	configengine.AssetMemory:      1.0,
	configengine.AssetKeybinding:  0.5,
}

var severityCoeff = map[Severity]float64{
	SeverityCritical: 4.0,
	SeverityHigh:     2.5,
	SeverityMedium:   1.5,
	SeverityLow:      0.5,
}

// ComputeHealth 按规格公式计算健康分。
func ComputeHealth(assets []configengine.Asset, findings []Finding) *HealthScore {
	// 资产权重总和
	totalW := 0.0
	wByID := map[string]float64{}
	nameByID := map[string]string{}
	typByID := map[string]configengine.AssetType{}
	for _, a := range assets {
		w := typeWeights[a.Type]
		if w == 0 {
			w = 1.0
		}
		totalW += w
		wByID[a.ID] = w
		nameByID[a.ID] = a.Name
		typByID[a.ID] = a.Type
	}
	if totalW == 0 {
		return &HealthScore{Score: 100, Band: band(100), Rmax: Rmax}
	}
	// 每资产风险(封顶 Rmax)
	risk := map[string]float64{}
	var ded []Deduction
	for _, f := range findings {
		w := wByID[f.AssetID]
		if w == 0 {
			w = 1.0
		}
		p := severityCoeff[f.Severity]
		if p == 0 {
			p = 0.5
		}
		risk[f.AssetID] += p
		ded = append(ded, Deduction{
			AssetID: f.AssetID, AssetType: string(f.AssetType),
			AssetName: nameByID[f.AssetID], RuleID: f.RuleID,
			Severity: f.Severity,
			Points:   p * w / (Rmax * totalW) * 100,
		})
	}
	num := 0.0
	for id, r := range risk {
		if r > Rmax {
			r = Rmax
		}
		num += r * wByID[id]
	}
	score := 100 * (1 - num/(Rmax*totalW))
	if score < 0 {
		score = 0
	}
	s := int(score + 0.5)
	return &HealthScore{Score: s, Band: band(s), Rmax: Rmax, Deductions: ded}
}

func band(score int) string {
	switch {
	case score >= 90:
		return "Excellent"
	case score >= 75:
		return "Good"
	case score >= 60:
		return "Fair"
	case score >= 40:
		return "At-Risk"
	default:
		return "Critical"
	}
}
