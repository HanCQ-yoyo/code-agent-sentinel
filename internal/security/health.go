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
	SeverityInfo:     0.0, // 低置信度 finding 不影响健康分
}

// findingWeight 返回单条 finding 的有效严重度系数。
// 未知 severity(map 中不存在)→ 兜底 0.5;info(显式 0.0)→ 保持 0.0。
// 抑制 finding 打 0.3 折(决策 #12:残值 30% 扣分)。
func findingWeight(f Finding) float64 {
	p, ok := severityCoeff[f.Severity]
	if !ok {
		p = 0.5 // 未知 severity 兜底
	}
	if f.Suppressed {
		p *= 0.3
	}
	return p
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
	// 每资产风险(封顶 Rmax)。
	// I-CORR-4:扣分必须可还原——Σ Deduction.Points == 100 − Score。
	// 旧实现用 pre-cap 的 p 算 Points,单资产风险超 Rmax 被封顶后,
	// ΣPoints > 100−Score(分数按封顶值算,扣分按原始值算)。
	// 修复分两遍:第一遍按资产累计原始风险 + 按资产分组 finding(保序);
	// 第二遍把每资产的扣分按其封顶后风险算出,再按各 finding 的 p 比例分配。
	risk := map[string]float64{}
	findingsByID := map[string][]Finding{}
	var order []string // 资产首次出现顺序,保证 Deductions 稳定
	for _, f := range findings {
		if _, ok := findingsByID[f.AssetID]; !ok {
			order = append(order, f.AssetID)
		}
		findingsByID[f.AssetID] = append(findingsByID[f.AssetID], f)
		p := findingWeight(f)
		risk[f.AssetID] += p
	}
	var ded []Deduction
	for _, id := range order {
		r := risk[id]
		if r > Rmax {
			r = Rmax
		}
		w := wByID[id]
		if w == 0 {
			w = 1.0
		}
		// 该资产实际(封顶后)扣分贡献。
		assetDeduction := r * w / (Rmax * totalW) * 100
		// 按各 finding 的 p 比例分配 assetDeduction:严重度高者占更大份额。
		var sumP float64
		for _, f := range findingsByID[id] {
			sumP += findingWeight(f)
		}
		for _, f := range findingsByID[id] {
			p := findingWeight(f)
			var points float64
			if sumP == 0 {
				// 理论不发生(p 默认 0.5);防御性:均分。
				points = assetDeduction / float64(len(findingsByID[id]))
			} else {
				points = assetDeduction * p / sumP
			}
			ded = append(ded, Deduction{
				AssetID: f.AssetID, AssetType: string(f.AssetType),
				AssetName: nameByID[f.AssetID], RuleID: f.RuleID,
				Severity: f.Severity,
				Points:   points,
			})
		}
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
