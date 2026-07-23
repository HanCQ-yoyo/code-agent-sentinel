// Package ruleengine special.go 实现三种特殊求值模式:
// repeat_check(重复串检测)、homoglyph_check(同形字/RTL 检测)、post_exclude(命中后排除)。
// 这些是求值器内置模式,非用户 op(validOp 不含它们),但在 evalLeaf 中可路由。
package ruleengine

import (
	"fmt"
	"strings"

	"code-agent-sentinel/internal/configengine"
)

// ── repeat_check ──

// evalRepeatCheck 扫描 content,检测连续重复的子串。
// 参数:
//   - minLen: 多字符模式的最低单元长度(单字符 run 始终检测,不受 minLen 限制)
//   - minRepeat: 最低重复次数(单元连续出现 ≥ minRepeat 次 → 命中)
//
// 算法分两层:
//  1. 单字符 run:同一字节连续出现 ≥ minRepeat 次(字符洪水,如 30 个 A)。
//     始终检测,不受 minLen 限制——单字符重复是最原始的注入模式。
//  2. 多字符模式:单元长度 ∈ [minLen, 20],连续重复 ≥ minRepeat 次。
//     对齐 SkillSpector MP 的 (.{2,20}?)\1{20,},但用 Go 代码而非正则实现。
//
// off-by-one 说明:原 PCRE `(.{2,20}?)\1{20,}` 要求单元 + 20 次反向引用 = 共 21 次。
// 本实现用 `count >= minRepeat`(默认 ≥20 次),比原 PCRE 多敏感 1 次(20 即触发,原需 21)。
// 对安全工具而言更敏感是更安全的方向(宁可多报不可漏报),故有意保留,不强行对齐 21。
//
// 参数约定:evalLeaf 路由时默认 minLen=2、minRepeat=20(对齐 SkillSpector);
// 可通过 rule.Metadata["repeat_min_len"] / ["repeat_min_repeat"] 覆盖。
func evalRepeatCheck(content string, minLen, minRepeat int) bool {
	if len(content) < minRepeat {
		return false // 连最低长度的单字符 run 都凑不够
	}
	if minLen < 1 {
		minLen = 1
	}

	// 1. 单字符 run 检测
	for i := 0; i < len(content); {
		j := i + 1
		for j < len(content) && content[j] == content[i] {
			j++
		}
		if j-i >= minRepeat {
			return true
		}
		i = j
	}

	// 2. 多字符模式检测:单元长度 [minLen, 20]
	maxUnit := 20
	if maxUnit > len(content) {
		maxUnit = len(content)
	}
	for unitLen := minLen; unitLen <= maxUnit; unitLen++ {
		for start := 0; start+unitLen <= len(content); start++ {
			unit := content[start : start+unitLen]
			count := 1
			pos := start + unitLen
			for pos+unitLen <= len(content) && content[pos:pos+unitLen] == unit {
				count++
				pos += unitLen
			}
			if count >= minRepeat {
				return true
			}
		}
	}

	return false
}

// ── homoglyph_check ──

// confusables 是 Cyrillic/Greek 常见同形字 → Latin 映射(约 40 条)。
// 键为 Unicode 码点,值为视觉上相似的 Latin 字符。
var confusables = map[rune]rune{
	// Cyrillic 小写
	'а': 'a', // а
	'е': 'e', // е
	'о': 'o', // о
	'р': 'p', // р
	'с': 'c', // с
	'х': 'x', // х
	'ѕ': 's', // ѕ
	'у': 'y', // у
	'и': 'u', // и (近似)
	'к': 'k', // к
	'м': 'm', // м
	'н': 'h', // н
	'т': 't', // т (近似 T)
	'ш': 'w', // ш (近似 w)
	'в': 'b', // в (近似 B/b)
	'г': 'r', // г (近似 r)
	// Cyrillic 大写
	'А': 'A', // А
	'Е': 'E', // Е
	'О': 'O', // О
	'Р': 'P', // Р
	'С': 'C', // С
	'Х': 'X', // Х
	'Ѕ': 'S', // Ѕ
	'У': 'Y', // У
	'К': 'K', // К
	'М': 'M', // М
	'Н': 'H', // Н
	'Т': 'T', // Т
	'В': 'B', // В
	// Greek 小写
	'ο': 'o', // ο
	'α': 'a', // α
	'ε': 'e', // ε
	'ρ': 'p', // ρ
	'τ': 't', // τ
	'ι': 'i', // ι
	'κ': 'k', // κ
	'χ': 'x', // χ
	'μ': 'm', // μ
	'ν': 'v', // ν
	'η': 'n', // η (近似 n)
	'υ': 'u', // υ
	// Greek 大写
	'Ο': 'O', // Ο
	'Α': 'A', // Α
	'Ε': 'E', // Ε
	'Ρ': 'P', // Ρ
	'Τ': 'T', // Τ
	'Ι': 'I', // Ι
	'Κ': 'K', // Κ
	'Χ': 'X', // Χ
	'Μ': 'M', // Μ
	'Ν': 'V', // Ν (近似 V)
}

// rtlCodepoints 是可导致文本方向反转的 Unicode 控制字符。
var rtlCodepoints = map[rune]string{
	'‮': "RLO", // RIGHT-TO-LEFT OVERRIDE
	'‭': "LRO", // LEFT-TO-RIGHT OVERRIDE
	'‎': "LRM", // LEFT-TO-RIGHT MARK
	'‏': "RLM", // RIGHT-TO-LEFT MARK
}

// evalHomoglyphCheck 检测 content 中的同形字(homoglyph)和 RTL 控制字符。
// 返回 (true, evidence) 当检测到任一:
//   - Cyrillic/Greek 同形字(confusables map 中的码点)
//   - RTL 方向控制字符(U+202E/U+202D/U+200E/U+200F)
//
// evidence 标注命中的码点,如 "同形字: U+0430(а→a); RTL: U+202E(RLO)"。
func evalHomoglyphCheck(content string) (bool, string) {
	var homoglyphs []string
	var rtls []string
	for _, r := range content {
		if latin, ok := confusables[r]; ok {
			homoglyphs = append(homoglyphs, fmt.Sprintf("U+%04X(%c→%c)", r, r, latin))
		}
		if name, ok := rtlCodepoints[r]; ok {
			rtls = append(rtls, fmt.Sprintf("U+%04X(%s)", r, name))
		}
	}

	var parts []string
	if len(homoglyphs) > 0 {
		parts = append(parts, "同形字: "+strings.Join(homoglyphs, ", "))
	}
	if len(rtls) > 0 {
		parts = append(parts, "RTL: "+strings.Join(rtls, ", "))
	}

	if len(parts) > 0 {
		return true, strings.Join(parts, "; ")
	}
	return false, ""
}

// ── post_exclude ──

// applyPostExclude 检查命中上下文是否匹配任一排除模式。
// hitCtx 匹配任一 pattern → 返回 true(应排除/降级)。
// 在 evalRegexMatch 中,regex 命中后若 rule.PostExclude 非空,
// 取命中片段(FindString 结果)作为 hitCtx 跑此函数;true 则降级为不报。
func applyPostExclude(hitCtx string, patterns []CompiledRegex) bool {
	for _, re := range patterns {
		if re.MatchString(hitCtx) {
			return true
		}
	}
	return false
}

// compilePostExcludePatterns 将 rule.PostExclude 字符串列表编译为正则列表。
// 使用 CompilePattern(含 regexp2 分流):post_exclude 来源于 dcg safe_pattern,
// 可能含 lookahead/lookbehind,故不能用纯 RE2 编译。
// 校验时已验证过,此处重建(Task 3 约定:post_exclude 不缓存)。
func compilePostExcludePatterns(rule *Rule) []CompiledRegex {
	pats := make([]CompiledRegex, 0, len(rule.PostExclude))
	for _, pat := range rule.PostExclude {
		re, err := compileRegexPattern(pat, rule.Dotall)
		if err != nil {
			continue // 校验时已报错,此处防御跳过
		}
		pats = append(pats, re)
	}
	return pats
}

// ── evalLeaf 路由辅助 ──

// evalSpecialRepeat 是 evalLeaf 对 op=repeat_check 的路由。
// 从 rule.Metadata 读取参数(默认 minLen=2/minRepeat=20),对字段值跑 evalRepeatCheck。
func evalSpecialRepeat(field string, a configengine.Asset, rule *Rule) (bool, string) {
	text, ok := fieldText(field, a)
	if !ok {
		return false, ""
	}

	minLen, minRepeat := 2, 20 // 默认值,对齐 SkillSpector MP
	if rule.Metadata != nil {
		if v := rule.Metadata["repeat_min_len"]; v != nil {
			if n, ok := toInt(v); ok && n > 0 {
				minLen = n
			}
		}
		if v := rule.Metadata["repeat_min_repeat"]; v != nil {
			if n, ok := toInt(v); ok && n > 0 {
				minRepeat = n
			}
		}
	}

	if evalRepeatCheck(text, minLen, minRepeat) {
		preview := text
		if len(preview) > 50 {
			preview = preview[:50] + "..."
		}
		return true, "重复串: " + preview
	}
	return false, ""
}

// evalSpecialHomoglyph 是 evalLeaf 对 op=homoglyph_check 的路由。
func evalSpecialHomoglyph(field string, a configengine.Asset) (bool, string) {
	text, ok := fieldText(field, a)
	if !ok {
		return false, ""
	}
	return evalHomoglyphCheck(text)
}

// fieldText 取字段的文本值:field=="content" → a.Content;否则 → a.Fields[field] stringify。
// 字段不存在 → ok=false。
func fieldText(field string, a configengine.Asset) (string, bool) {
	if field == "content" {
		if a.Content != "" {
			return a.Content, true
		}
		return "", false
	}
	v, ok := a.Fields[field]
	if !ok {
		return "", false
	}
	return stringify(v), true
}

// toInt 从 any 类型提取 int(YAML 解析后数值通常为 float64)。
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}
