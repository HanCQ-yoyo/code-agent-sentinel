package ruleengine

import (
	"encoding/json"
	"fmt"
	"strings"

	"code-agent-sentinel/internal/configengine"
)

// Eval 对单个资产求值规则。返回 EvalResult(命中 + 证据 + 命中位置)。
// 禁用规则(Match.raw 为 nil)返回零值 EvalResult。
// 仅 content 字段的 regex_match/contains 叶子产生 Location;not_*、字段级匹配、
// 反混淆 candidate 命中不产位置。命中位置不进入 Fingerprint(见 fingerprint.go)。
func Eval(rule Rule, a configengine.Asset) EvalResult {
	if len(rule.Match.raw) == 0 {
		return EvalResult{} // 禁用
	}
	var locs []Location
	matched, evidence := evalMatch(rule.Match.raw, a, &rule, &locs)
	return EvalResult{Matched: matched, Evidence: evidence, Locations: locs}
}

// evalMatch 递归求值 match 节点。node 是 raw map(布尔节点或叶子)。
// locs 用于收集 content 字段叶子命中位置;NOT 子树传 nil(取反不产位置)。
func evalMatch(node map[string]any, a configengine.Asset, rule *Rule, locs *[]Location) (bool, string) {
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
			// 记录本 child 求值前的 locs 长度:child 若成功会向 *locs 追加合法位置;
			// child 若失败,须截断它在失败前可能已追加的部分位置(content 叶子先命中、
			// 后续兄弟叶子失败 → AND 短路,但这些虚假位置不能留给父级,否则 OR 成功分支
			// 会带上失败兄弟路径的位置 → 过度高亮错误行)。
			start := len(*locs)
			child, ok := it.(map[string]any)
			if !ok {
				*locs = (*locs)[:start]
				return false, ""
			}
			ok2, e := evalMatch(child, a, rule, locs)
			if !ok2 {
				*locs = (*locs)[:start] // 截断本 child 失败前已追加的虚假位置
				return false, ""        // 短路
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
			// 每个 child 用独立局部缓冲:失败 child 的虚假位置天然隔离在 childLocs,
			// 不会污染父级 *locs;仅成功 child 的位置合并进父级。彻底消除失败兄弟路径
			// 污染(AND 截断已处理 AND 内部,此处处理 OR 层 sibling 隔离)。
			var childLocs []Location
			ok2, e := evalMatch(child, a, rule, &childLocs)
			if ok2 {
				*locs = append(*locs, childLocs...)
				return true, e // 短路:取命中者的 evidence + locations
			}
		}
		return false, ""
	}

	if v, has := node["not"]; has {
		child, ok := v.(map[string]any)
		if !ok {
			return false, ""
		}
		// NOT 语义:子节点命中位置不外传(取反 = 无命中点)
		ok2, e := evalMatch(child, a, rule, nil)
		return !ok2, "NOT(" + e + ")"
	}

	// 叶子节点
	return evalLeaf(node, a, rule, locs)
}

// evalLeaf 求值叶子节点(field + op + value)。
// field=="content" 取 a.Content;否则取 a.Fields[field]。
// 字段缺失:exists→false / not_exists→true / 其它→false。
// locs 非 nil 时,content 字段的 contains/regex_match 会向其追加命中位置。
func evalLeaf(node map[string]any, a configengine.Asset, rule *Rule, locs *[]Location) (bool, string) {
	fieldStr, ok := node["field"].(string)
	if !ok {
		return false, ""
	}
	opStr, ok := node["op"].(string)
	if !ok {
		return false, ""
	}

	// 特殊求值模式(repeat_check / homoglyph_check):非用户 op,但 evalLeaf 可路由
	if opStr == SpecialRepeat {
		return evalSpecialRepeat(fieldStr, a, rule)
	}
	if opStr == SpecialHomoglyph {
		return evalSpecialHomoglyph(fieldStr, a)
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
		return evalContains(fieldVal, node["value"], fieldStr, locs)

	case OpNotContains:
		matched, _ := evalContains(fieldVal, node["value"], fieldStr, nil) // not 不产位置
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
		return evalRegexMatch(fieldStr, fieldVal, node["value"], rule, OpRegexMatch, locs)

	case OpNotRegexMatch:
		matched, ev := evalRegexMatch(fieldStr, fieldVal, node["value"], rule, OpNotRegexMatch, nil) // not 不产位置
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
// 支持 []any(YAML 解析默认)和 []string(configengine parse_settings/parse_mcp 的真实类型)。
// fieldStr + locs 用于位置计算:仅 fieldStr=="content" 且 locs 非 nil 时,slice 之外的标量命中
// 会向 locs 追加所有命中区间(字段级匹配 locs 为 nil,不算)。
func evalContains(fieldVal any, value any, fieldStr string, locs *[]Location) (bool, string) {
	valStr, ok := value.(string)
	if !ok {
		return false, ""
	}
	// configengine 把 permissions allow/deny/ask 存为 []string(非 []any),
	// mcp args 也是 []string。逐元素检查,而非 stringify 整个 slice 后子串匹配
	// (后者会因 fmt.Sprint 在元素间插入空格而跨元素边界误命中)。
	switch arr := fieldVal.(type) {
	case []any:
		for _, elem := range arr {
			s := stringify(elem)
			if s == valStr || strings.Contains(s, valStr) {
				return true, s
			}
		}
		return false, ""
	case []string:
		for _, elem := range arr {
			if elem == valStr || strings.Contains(elem, valStr) {
				return true, elem
			}
		}
		return false, ""
	}
	s := stringify(fieldVal)
	if strings.Contains(s, valStr) {
		// content 字段:算所有命中位置(字段级匹配 locs 为 nil,不算)
		if fieldStr == "content" && locs != nil {
			*locs = append(*locs, locationsFromOffsets(s, containsAllOffsets(s, valStr))...)
		}
		return true, valStr
	}
	return false, ""
}

// evalRegexMatch:用 rule.regexes 缓存(键 op:field:value)查找编译好的正则;
// 若缓存未命中(未跑 Validate),惰性编译并缓存。
// op 参数用于缓存键区分 regex_match 与 not_regex_match(validate 按 op 分别存储)。
// 若 rule.Deobfuscation 非空,对每个反混淆 candidate 跑正则(作用于任意字段),任一命中即 matched。
// locs 非 nil 时,field=="content" 的原始文本命中会向其追加所有未排除命中区间;
// 反混淆 candidate 命中不映射回原文,不产位置。
func evalRegexMatch(field string, fieldVal any, value any, rule *Rule, op string, locs *[]Location) (bool, string) {
	pattern, ok := value.(string)
	if !ok {
		return false, ""
	}
	re := lookupRegex(rule, op, field, pattern)
	if re == nil {
		return false, ""
	}

	text := stringify(fieldVal)

	// post_exclude 编译(若规则有 post_exclude 模式)
	var excludePats []CompiledRegex
	if len(rule.PostExclude) > 0 {
		excludePats = compilePostExcludePatterns(rule)
	}

	// 无反混淆:对原始文本跑(首个未排除命中 + 全部未排除命中偏移)。
	// Finding #1 修复:旧实现用 FindString 只取最左匹配,若该匹配被 post_exclude 排除就
	// 直接放弃,从不检查后续匹配 → 漏报(如 PE2 对 "sudo -v && sudo rm" 漏报 sudo rm)。
	if hit, indices, ok := firstNonExcludedHit(re, text, excludePats); ok {
		// content 字段原始文本命中:算位置(反混淆 candidate 命中不映射回原文,不算)
		if field == "content" && locs != nil {
			*locs = append(*locs, locationsFromOffsets(text, indices)...)
		}
		return true, hit
	}

	// 有反混淆:对每个 candidate 跑(不链式)。
	// Task 14 修复:移除 field=="content" 限制 — deobfuscation 应作用于任意字段
	// (skill rules 需要对 description 字段做反混淆)。原有 field==content 规则不受影响。
	// candidate 偏移不映射回原文 → 不产 location。
	if len(rule.Deobfuscation) > 0 {
		candidates := Deobfuscate(text, rule.Deobfuscation)
		for _, c := range candidates {
			if c.Method == "" {
				continue // 原始文本,已跑过
			}
			// post_exclude 同样作用于反混淆命中(遍历全部匹配)
			if hit, _, ok := firstNonExcludedHit(re, c.Text, excludePats); ok {
				return true, fmt.Sprintf("[%s] %s", c.Method, hit)
			}
		}
	}

	return false, ""
}

// firstNonExcludedHit 在 text 上迭代正则全部匹配,返回首个未被 post_exclude 排除的命中串
// 及所有未排除命中的字节区间 [start,end)。全部被排除(或无匹配)才 ok=false。
// Finding #1:旧实现只取最左匹配(FindString),若被 post_exclude 排除就放弃 → 漏报后续匹配。
// 现遍历 FindAllStringIndex 的所有匹配,收集未排除的命中区间;首个未排除的作为 first。
// post_exclude 缺省时 excludePats 为空 → applyPostExclude 恒 false → 首个匹配命中,与旧行为一致。
func firstNonExcludedHit(re CompiledRegex, text string, excludePats []CompiledRegex) (string, [][2]int, bool) {
	var first string
	var indices [][2]int
	for _, idx := range re.FindAllStringIndex(text) {
		hit := text[idx[0]:idx[1]]
		if applyPostExclude(hit, excludePats) {
			continue
		}
		if first == "" {
			first = hit
		}
		indices = append(indices, [2]int{idx[0], idx[1]})
	}
	return first, indices, first != ""
}

// evalKeyMatches:字段值是 map,对其 KEY 跑正则。
// 接受 map[string]any 和 map[string]string(后者是 settings 解析器对 env 的存储类型)。
func evalKeyMatches(field string, fieldVal any, value any, rule *Rule) (bool, string) {
	pattern, ok := value.(string)
	if !ok {
		return false, ""
	}
	// 提取 map 的键集合:兼容 map[string]any 和 map[string]string
	var keys []string
	switch m := fieldVal.(type) {
	case map[string]any:
		for k := range m {
			keys = append(keys, k)
		}
	case map[string]string:
		for k := range m {
			keys = append(keys, k)
		}
	default:
		return false, ""
	}
	re := lookupRegex(rule, OpKeyMatches, field, pattern)
	if re == nil {
		return false, ""
	}
	for _, k := range keys {
		if re.MatchString(k) {
			return true, k
		}
	}
	return false, ""
}

// evalWithin:字段值是 slice → 所有元素都在 value 数组内(subset);
// 字段值是标量 → 标量等于 value 数组中某个元素。
// 支持 []any(YAML 解析默认)和 []string(configengine parse_mcp 的 args 真实类型)。
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

	// configengine 把 mcp args 存为 []string(非 []any),permissions allow 也是 []string。
	// 逐元素检查 subset,而非 stringify 整个 slice(后者产生 "[a b]" 永远不在白名单)。
	switch fieldArr := fieldVal.(type) {
	case []any:
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
	case []string:
		for _, elem := range fieldArr {
			found := false
			for _, w := range whitelist {
				if elem == w {
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
func lookupRegex(rule *Rule, op, field, pattern string) CompiledRegex {
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
		rule.regexes = make(map[string]CompiledRegex)
	}
	rule.regexes[key] = re
	return re
}

// stringify 将任意值转为字符串。
func stringify(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case json.RawMessage:
		return string(val)
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

// ── Location 位置计算辅助(Task 4)──

// lineStartOffsets 返回每行起始字节偏移(starts[i] = 第 i+1 行起点)。第 1 行起点 0。
func lineStartOffsets(text string) []int {
	starts := []int{0}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// offsetLineCol 把字节偏移转成 1-based (行号, 列号)。starts 来自 lineStartOffsets。
func offsetLineCol(starts []int, offset int) (int, int) {
	// 二分找最大 starts[i] <= offset
	lo, hi := 0, len(starts)-1
	lineIdx := 0
	for lo <= hi {
		mid := (lo + hi) / 2
		if starts[mid] <= offset {
			lineIdx = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return lineIdx + 1, offset - starts[lineIdx] + 1
}

// locationsFromOffsets 把字节区间 [start,end) 列表转成 Location(基于 text 换行结构),最多 50 个。
// EndCol 为半开区间末列:末字节列 +1 后再 +1(因 offsetLineCol 返回 1-based 列,
// end-1 指向命中末字节,其列 +1 即为 EndCol 的半开边界)。
func locationsFromOffsets(text string, offsets [][2]int) []Location {
	if len(offsets) == 0 {
		return nil
	}
	starts := lineStartOffsets(text)
	out := make([]Location, 0, len(offsets))
	for _, o := range offsets {
		if len(out) >= 50 {
			break
		}
		line, startCol := offsetLineCol(starts, o[0])
		_, endCol := offsetLineCol(starts, o[1]-1) // end exclusive → 末字节列 +1
		out = append(out, Location{Line: line, StartCol: startCol, EndCol: endCol + 1})
	}
	return out
}

// containsAllOffsets 返回 sub 在 s 中所有非重叠出现的字节区间 [start,end),最多 50 个。
func containsAllOffsets(s, sub string) [][2]int {
	var out [][2]int
	if sub == "" {
		return out
	}
	start := 0
	for {
		i := strings.Index(s[start:], sub)
		if i < 0 {
			break
		}
		b := start + i
		out = append(out, [2]int{b, b + len(sub)})
		start = b + len(sub)
		if len(out) >= 50 {
			break
		}
	}
	return out
}
