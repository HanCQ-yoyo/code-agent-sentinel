package security

import (
	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security/ruleengine"
)

// dedup.go — 资产内去重(Task 7)。
//
// 同资产内同位置(content 同行 / command 字段同串)的多条 finding 合并为一条 finding-group。
// group:RuleID=最高 severity 的规则(并列取首个),ContributingRuleIDs=其余规则,
// Locations=并集,Severity=最大,Evidence/Message/Fingerprint/Remediation 取主规则。
//
// 主键:
//   - content 字段命中(Locations 非空):asset_id + Locations[0].Line(同行合并)。
//   - command 字段命中(Locations 空,AssetType=hook/mcp_server):asset_id + Evidence
//     (命令串相同才合并;Evidence 空 → 兜底用 AssetName,避免无 Evidence 的不同 finding 误并)。
//   - 其他(Locations 空 + 非 hook/mcp_server):不合并,唯一负 key 保留。
//
// 不合并的情况:
//   - 跨资产(AssetID 不同):不合并(那是聚合视图的事)。
//   - combo finding / load-error finding:在 Scan 中 dedupIntraAsset 调用之后才 append,
//     不参与去重(它们各自语义独立)。
//
// 设计意图:避免同位置多规则命中在 UI/健康分中重复扣分。合并后 ContributingRuleIDs
// 记录其他贡献规则,UI 可展示"X + N 条相关规则命中"。
func dedupIntraAsset(findings []Finding) []Finding {
	type key struct {
		assetID string
		line    int    // content 命中:>0;command 命中:0
		cmd     string // command 命中时用命令串作 key
	}
	groups := map[key][]int{} // key → findings 索引
	order := []key{}

	for i, f := range findings {
		var k key
		k.assetID = f.AssetID
		if len(f.Locations) > 0 {
			// content 字段命中:按行号合并(同行多规则 → 1 group)。
			k.line = f.Locations[0].Line
		} else if f.AssetType == configengine.AssetHook || f.AssetType == configengine.AssetMCPServer {
			// command 字段命中:用命令串(Evidence)作 key。
			// Evidence 是 string(不是 interface),不能用 type assertion(修 brief 的 bug)。
			if f.Evidence != "" {
				k.cmd = f.Evidence
			} else {
				k.cmd = f.AssetName // 无 Evidence 时兜底,避免误并(不同 finding 同名才合并,极少)
			}
		} else {
			// 无位置且非命令资产(理论少,如 load-error):不合并,唯一负 key 保留。
			k.line = -(i + 1)
			if _, exists := groups[k]; !exists {
				order = append(order, k)
			}
			groups[k] = []int{i}
			continue
		}
		if _, exists := groups[k]; !exists {
			order = append(order, k)
		}
		groups[k] = append(groups[k], i)
	}

	out := make([]Finding, 0, len(order))
	for _, k := range order {
		idxs := groups[k]
		if len(idxs) == 1 {
			out = append(out, findings[idxs[0]])
			continue
		}
		// 选主规则:最大 severity(并列取首个)。
		primaryIdx := idxs[0]
		for _, idx := range idxs[1:] {
			if severityRank(findings[idx].Severity) > severityRank(findings[primaryIdx].Severity) {
				primaryIdx = idx
			}
		}
		group := findings[primaryIdx]
		var contributing []string
		locs := append([]ruleengine.Location(nil), group.Locations...)
		for _, idx := range idxs {
			if idx == primaryIdx {
				continue
			}
			contributing = append(contributing, findings[idx].RuleID)
			locs = append(locs, findings[idx].Locations...)
		}
		group.ContributingRuleIDs = contributing
		group.Locations = locs
		out = append(out, group)
	}
	return out
}

// severityRank 返回 severity 排序权重(critical>high>medium>low>info)。
// 用于去重时选主规则:同位置多规则命中,取最大 severity 的规则作 group 的 RuleID/Severity。
func severityRank(s Severity) int {
	switch s {
	case SeverityCritical:
		return 5
	case SeverityHigh:
		return 4
	case SeverityMedium:
		return 3
	case SeverityLow:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}
