package ruleengine

import "fmt"

// Syntax 返回规则 match 树的可读语法摘要,供 UI(Meta.Rules[].Syntax)展示。
//
// 叶子节点形如 `field op value`:
//   - regex_match / not_regex_match / key_matches → `field op /pattern/`
//   - contains / not_contains / eq / not_equals  → `field op "value"`
//   - within / not_within                        → `field op [a,b,c]`
//   - exists / not_exists                        → `field op`
//
// 布尔节点(and/or/not)递归取首个子节点的语法——与 Fingerprint 的 ruleAnchor
// 取首子锚点一致,保证同一规则的 Syntax 稳定。禁用规则(Match 为空)返回空串。
//
// 放在 ruleengine 包是因为 MatchNode.raw 未导出,security 包的 RulesDetector.Meta
// 无法直接读 match 树;由拥有 match 树知识的 ruleengine 提供展示用语法最合适。
// (Task 11:旧 baseline.go 的 baselineSyntax / injection.go 的 r.Pattern 两套展示,
// 统一引擎后由本方法单一来源生成。)
func (r Rule) Syntax() string {
	field, op, anchor := ruleAnchor(r.Match.raw)
	if op == "" {
		return ""
	}
	switch op {
	case OpRegexMatch, OpNotRegexMatch, OpKeyMatches:
		return fmt.Sprintf("%s %s /%s/", field, op, anchor)
	case OpExists, OpNotExists:
		return fmt.Sprintf("%s %s", field, op)
	case OpWithin, OpNotWithin:
		return fmt.Sprintf("%s %s [%s]", field, op, anchor)
	default:
		// contains / not_contains / eq / not_equals —— anchor 是规则声明的 value 字符串
		return fmt.Sprintf("%s %s %q", field, op, anchor)
	}
}
