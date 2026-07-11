// 本文件放在 security 包(而非 suppression 子包),因为 applySuppression 需要修改
// *Finding 字段,而 Finding 定义在 security 包。若放在 suppression 包会产生循环依赖:
// suppression → security(为 *Finding)→ suppression(RulesDetector 调用 applySuppression)。
// 与 ruleengine 子包同理:ruleengine 也不 import security,改取 asset_id string 等原语。
// 因此 suppression 子包保持纯净(只提供 BaselineSet / Suppressions 数据类型与匹配逻辑),
// Finding 变异助手留在 security 包。
package security

import "code-agent-sentinel/internal/security/suppression"

// applySuppression 对单条 Finding 施加两层抑制:
//  1. baseline 命中 → Suppressed=true, Suppression="baseline", Reason=""(baseline 不填 Reason)
//  2. 行内豁免命中 → Suppressed=true, Suppression="inline", Reason=匹配到的 Reason
//  3. 均不命中 → Finding 不变
//
// baseline 和 supprs 为 nil 时安全跳过(不 panic)。
func applySuppression(f *Finding, fp string, baseline *suppression.BaselineSet, supprs *suppression.Suppressions) {
	if baseline != nil && baseline.Contains(fp) {
		f.Suppressed = true
		f.Suppression = "baseline"
		f.Reason = ""
		return
	}
	if supprs != nil {
		if suppressed, reason := supprs.Match(f.RuleID, f.AssetID, fp); suppressed {
			f.Suppressed = true
			f.Suppression = "inline"
			f.Reason = reason
		}
	}
}
