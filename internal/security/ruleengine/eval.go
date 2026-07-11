package ruleengine

import (
	"fmt"
	"regexp"
	"strings"

	"code-agent-sentinel/internal/configengine"
)

// Eval 对单个资产求值规则。返回是否命中 + 人类可读的 evidence。
// 禁用规则(Match.raw 为 nil)返回 (false, "")。
func Eval(rule Rule, a configengine.Asset) (bool, string) {
	if len(rule.Match.raw) == 0 {
		return false, "" // 禁用
	}
	return evalMatch(rule.Match.raw, a, &rule)
}

// evalMatch 递归求值 match 节点。node 是 raw map(布尔节点或叶子)。
func evalMatch(node map[string]any, a configengine.Asset, rule *Rule) (bool, string) {
	if len(node) == 0 {
		return false, ""
	}

	// 检测布尔操作符
	if v, has := node["and"]; has {
		items, ok := v.([]any)
		if !ok {
			return false, ""
		}
		var evs []string
		for _, it := range items {
			child, ok := it.(map[string]any)
			if !ok {
				return false, ""
			}
			ok2, e := evalMatch(child, a, rule)
			if !ok2 {
				return false, "" // 短路
			}
			if e != "" {
				evs = append(evs, e)
			}
		}
		return true, strings.Join(evs, " 且 ")
	}

	if v, has := node["or"]; has {
		items, ok := v.([]any)
		if !ok {
			return false, ""
		}
		for _, it := range items {
			child, ok := it.(map[string]any)
			if !ok {
				continue
			}
			ok2, e := evalMatch(child, a, rule)
			if ok2 {
				return true, e // 短路:取命中者的 evidence
			}
		}
		return false, ""
	}

	if v, has := node["not"]; has {
		child, ok := v.(map[string]any)
		if !ok {
			return false, ""
		}
		ok2, e := evalMatch(child, a, rule)
		return !ok2, "NOT(" + e + ")"
	}

	// 叶子节点
	return evalLeaf(node, a, rule)
}

// evalLeaf 求值叶子节点(field + op + value)。
// field=="content" 取 a.Content;否则取 a.Fields[field]。
// 字段缺失:exists→false / not_exists→true / 其它→false。
func evalLeaf(node map[string]any, a configengine.Asset, rule *Rule) (bool, string) {
	fieldStr, ok := node["field"].(string)
	if !ok {
		return false, ""
	}
	opStr, ok := node["op"].(string)
	if !ok {
		return false, ""
	}

	// 特殊求值模式(Task 5 实现)
	if opStr == SpecialRepeat || opStr == SpecialHomoglyph {
		// TODO(Task 5): 实现 repeat_check / homoglyph_check
		return false, ""
	}

	// 取字段值
	var fieldVal any
	var fieldExists bool
	if fieldStr == "content" {
		if a.Content != "" {
			fieldVal = a.Content
			fieldExists = true
		}
	} else {
		fieldVal, fieldExists = a.Fields[fieldStr]
	}

	// 字段缺失语义
	if !fieldExists {
		switch opStr {
		case OpNotExists:
			return true, fmt.Sprintf("field %q 不存在", fieldStr)
		case OpExists:
			return false, ""
		default:
			return false, "" // 其它 op 在字段缺失时不命中
		}
	}

	// 字段存在时的 op 分支
	switch opStr {
	case OpExists:
		return true, fmt.Sprintf("field %q 存在", fieldStr)

	case OpNotExists:
		return false, "" // 字段存在,not_exists 不命中

	case OpContains:
		return evalContains(fieldVal, node["value"])

	case OpNotContains:
		matched, _ := evalContains(fieldVal, node["value"])
		return !matched, fmt.Sprintf("field %q 不含 %v", fieldStr, node["value"])

	case OpEq:
		valStr, _ := node["value"].(string)
		s := stringify(fieldVal)
		return s == valStr, fmt.Sprintf("%s == %q", fieldStr, s)

	case OpNotEquals:
		valStr, _ := node["value"].(string)
		s := stringify(fieldVal)
		return s != valStr, fmt.Sprintf("%s != %q", fieldStr, valStr)

	case OpRegexMatch:
		return evalRegexMatch(fieldStr, fieldVal, node["value"], rule)

	case OpNotRegexMatch:
		matched, ev := evalRegexMatch(fieldStr, fieldVal, node["value"], rule)
		return !matched, fmt.Sprintf("NOT(%s)", ev)

	case OpKeyMatches:
		return evalKeyMatches(fieldStr, fieldVal, node["value"], rule)

	case OpWithin:
		return evalWithin(fieldVal, node["value"])

	case OpNotWithin:
		matched, _ := evalWithin(fieldVal, node["value"])
		return !matched, fmt.Sprintf("field %q 有元素不在白名单内", fieldStr)

	default:
		return false, ""
	}
}

// evalContains:字段值是 slice → 检查成员(任一元素 stringify 后含 value,或等于 value);
// 非 slice → stringify 后子串检查。
func evalContains(fieldVal any, value any) (bool, string) {
	valStr, ok := value.(string)
	if !ok {
		return false, ""
	}
	if arr, isSlice := fieldVal.([]any); isSlice {
		for _, elem := range arr {
			s := stringify(elem)
			if s == valStr || strings.Contains(s, valStr) {
				return true, s
			}
		}
		return false, ""
	}
	s := stringify(fieldVal)
	if strings.Contains(s, valStr) {
		return true, valStr
	}
	return false, ""
}

// evalRegexMatch:用 rule.regexes 缓存(键 op:field:value)查找编译好的正则;
// 若缓存未命中(未跑 Validate),惰性编译并缓存。
// 若 rule.Deobfuscation 非空且 field==content,对每个反混淆 candidate 跑正则,任一命中即 matched。
func evalRegexMatch(field string, fieldVal any, value any, rule *Rule) (bool, string) {
	pattern, ok := value.(string)
	if !ok {
		return false, ""
	}
	re := lookupRegex(rule, OpRegexMatch, field, pattern)
	if re == nil {
		return false, ""
	}

	// 无反混淆:直接对原始文本跑
	text := stringify(fieldVal)
	if matched := re.FindString(text); matched != "" {
		return true, matched
	}

	// 有反混淆且 field==content:对每个 candidate 跑(不链式)
	if len(rule.Deobfuscation) > 0 && field == "content" {
		candidates := Deobfuscate(text, rule.Deobfuscation)
		for i, c := range candidates {
			if i == 0 {
				continue // out[0] 是原始文本,已跑过
			}
			if m := re.FindString(c); m != "" {
				method := rule.Deobfuscation[i-1]
				return true, fmt.Sprintf("[%s] %s", method, m)
			}
		}
	}

	return false, ""
}

// evalKeyMatches:字段值是 map,对其 KEY 跑正则。
func evalKeyMatches(field string, fieldVal any, value any, rule *Rule) (bool, string) {
	pattern, ok := value.(string)
	if !ok {
		return false, ""
	}
	m, ok := fieldVal.(map[string]any)
	if !ok {
		return false, ""
	}
	re := lookupRegex(rule, OpKeyMatches, field, pattern)
	if re == nil {
		return false, ""
	}
	for k := range m {
		if re.MatchString(k) {
			return true, k
		}
	}
	return false, ""
}

// evalWithin:字段值是 slice → 所有元素都在 value 数组内(subset);
// 字段值是标量 → 标量等于 value 数组中某个元素。
func evalWithin(fieldVal any, value any) (bool, string) {
	arr, ok := value.([]any)
	if !ok {
		return false, ""
	}
	// 构建 value 集合
	whitelist := make([]string, 0, len(arr))
	for _, v := range arr {
		whitelist = append(whitelist, stringify(v))
	}

	if fieldArr, isSlice := fieldVal.([]any); isSlice {
		for _, elem := range fieldArr {
			s := stringify(elem)
			found := false
			for _, w := range whitelist {
				if s == w {
					found = true
					break
				}
			}
			if !found {
				return false, ""
			}
		}
		return true, fmt.Sprintf("all in [%s]", strings.Join(whitelist, ", "))
	}

	// 标量:检查是否在数组内
	s := stringify(fieldVal)
	for _, w := range whitelist {
		if s == w {
			return true, s
		}
	}
	return false, ""
}

// lookupRegex 按 op:field:value 键从 rule.regexes 取编译好的正则;
// 若未命中(规则未经 Validate),惰性编译并缓存。
func lookupRegex(rule *Rule, op, field, pattern string) *regexp.Regexp {
	key := op + ":" + field + ":" + pattern
	if rule.regexes != nil {
		if re, ok := rule.regexes[key]; ok {
			return re
		}
	}
	// 惰性编译
	re, err := compileRegexPattern(pattern, rule.Dotall)
	if err != nil {
		return nil
	}
	if rule.regexes == nil {
		rule.regexes = make(map[string]*regexp.Regexp)
	}
	rule.regexes[key] = re
	return re
}

// stringify 将任意值转为字符串。
func stringify(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}
