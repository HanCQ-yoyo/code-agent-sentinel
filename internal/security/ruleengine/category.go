package ruleengine

import "strings"

// Category 是 finding 的治理分类(问题域),用于细粒度筛选与按域运营。
// 派生自 rule_id 前缀(确定性,不进 ScanRecord,API 读时 attach)。
type Category string

const (
	CategoryDestructiveCommand Category = "destructive_command" // 危险命令
	CategoryPromptInjection    Category = "prompt_injection"    // 提示注入
	CategoryBaseline           Category = "baseline"            // 配置基线
	CategorySkillAbuse         Category = "skill_abuse"         // 技能滥用
	CategoryCombo              Category = "combo"               // 组合风险
	CategoryLoadError          Category = "load_error"          // 规则加载错误
	CategoryUnknown            Category = "unknown"
)

// CategoryOf 由 rule_id 派生治理分类。
// destructive.* → 危险命令;injection.* → 提示注入;baseline.* → 配置基线;
// skill.* → 技能滥用;combo.* → 组合风险;rules.load-error → 加载错误;其余 → unknown。
func CategoryOf(ruleID string) Category {
	switch {
	case ruleID == "rules.load-error":
		return CategoryLoadError
	case strings.HasPrefix(ruleID, "destructive."):
		return CategoryDestructiveCommand
	case strings.HasPrefix(ruleID, "injection."):
		return CategoryPromptInjection
	case strings.HasPrefix(ruleID, "baseline."):
		return CategoryBaseline
	case strings.HasPrefix(ruleID, "skill."):
		return CategorySkillAbuse
	case strings.HasPrefix(ruleID, "combo."):
		return CategoryCombo
	default:
		return CategoryUnknown
	}
}

// InjectionSubclass 提取 injection 规则的威胁子类(rule_id 第二段)。
// 如 "injection.data-exfiltration.x1" → "data-exfiltration"。非 injection 返回空。
// 前端字典可据此展子类标签。
func InjectionSubclass(ruleID string) string {
	if !strings.HasPrefix(ruleID, "injection.") {
		return ""
	}
	rest := strings.TrimPrefix(ruleID, "injection.")
	idx := strings.Index(rest, ".")
	if idx < 0 {
		return rest
	}
	return rest[:idx]
}
