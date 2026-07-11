package ruleengine

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

// Fingerprint 计算一条规则在一个资产上的确定性指纹(sha256 hex,64 字符)。
//
// 指纹锚定**规则意图**(rule_id + field + op + 规则声明的 value/pattern),
// 不含命中子串、行号、资产内容或 evidence 展示文本。这保证 baseline 跨运行
// 稳定:资产内容微调(但不改变触发的规则/field/意图锚点)→ 指纹不变。
//
// 公式(spec 决策 #13):
//
//	fingerprint = sha256(rule_id + ":" + asset_id + ":" + evidence_normalized)
//	evidence_normalized = rule_id + ":" + field + ":" + op + ":" + rule_anchor
func Fingerprint(rule Rule, assetID string) string {
	return sha256hex(rule.ID + ":" + assetID + ":" + evidenceNormalized(rule))
}

// evidenceNormalized 提取规则意图锚点并拼成规范化字符串。
// 禁用规则(Match.raw 为 nil)→ ruleAnchor 返回空三元组,结果为 "rule_id:::"。
func evidenceNormalized(rule Rule) string {
	field, op, anchor := ruleAnchor(rule.Match.raw)
	return rule.ID + ":" + field + ":" + op + ":" + anchor
}

// ruleAnchor 从 match 节点提取 (field, op, anchor)。
// 布尔节点(and/or/not)递归取首个子节点的锚点;叶子按 op 类别提取 value/pattern。
// node 为 any 因为布尔子节点可能类型不匹配,需防御性处理(返回空串,不 panic)。
func ruleAnchor(node any) (field, op, anchor string) {
	m, ok := node.(map[string]any)
	if !ok || len(m) == 0 {
		return "", "", ""
	}

	// 布尔节点:取首个子节点锚点(与 eval.go 的 and/or/not 检测顺序一致)
	if v, has := m["and"]; has {
		return firstChildAnchor(v)
	}
	if v, has := m["or"]; has {
		return firstChildAnchor(v)
	}
	if v, has := m["not"]; has {
		if child, ok := v.(map[string]any); ok {
			return ruleAnchor(child)
		}
		return "", "", ""
	}

	// 叶子节点
	field, _ = m["field"].(string)
	op, _ = m["op"].(string)
	anchor = leafAnchor(op, m["value"])
	return field, op, anchor
}

// firstChildAnchor 从 and/or 的 []any 子节点列表取首个 map 子节点的锚点。
func firstChildAnchor(v any) (field, op, anchor string) {
	items, ok := v.([]any)
	if !ok || len(items) == 0 {
		return "", "", ""
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		return "", "", ""
	}
	return ruleAnchor(first)
}

// leafAnchor 按 op 类别从 value 提取规则意图锚点:
//   - regex_match / not_regex_match / key_matches → pattern 字符串(规则声明的正则原文)
//   - contains / not_contains / eq / not_equals   → value 字符串(规则声明的匹配值)
//   - within / not_within                          → value 数组排序后逗号拼接(规范化顺序)
//   - exists / not_exists / 未知 op                → 空串
func leafAnchor(op string, value any) string {
	switch op {
	case OpRegexMatch, OpNotRegexMatch, OpKeyMatches,
		OpContains, OpNotContains, OpEq, OpNotEquals:
		return stringify(value)

	case OpWithin, OpNotWithin:
		arr, ok := value.([]any)
		if !ok {
			return ""
		}
		parts := make([]string, 0, len(arr))
		for _, v := range arr {
			parts = append(parts, stringify(v))
		}
		sort.Strings(parts) // 排序使不同顺序的同集数组产生相同锚点
		return joinComma(parts)

	default:
		return "" // exists / not_exists / 未知 op
	}
}

// joinComma 用逗号拼接字符串切片(无空格)。
func joinComma(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += "," + p
	}
	return out
}

// sha256hex 返回输入的 sha256 十六进制摘要(64 字符)。
func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
