package ruleengine

import "strings"

// negationWords 是会翻转语义的否定/禁止词。
// 命中点所在行行首(忽略前导空白)或匹配点前 lookBehindChars 字符内出现 → 抑制。
var negationWords = []string{
	"禁止", "不允许", "请勿", "不要", "切勿", "不可",
	"never", "do not", "don't", "avoid", "should not", "must not",
}

const lookBehindChars = 40

// IsNegatedByContext 判断一条 content 字段命中是否被否定上下文抑制。
//
// 规则:
//   - locs 为空(command 字段命中无行位置)→ 不抑制(避免假阴性:
//     `禁止: rm -rf` 注释里的否定词不能让命令变安全)。
//   - 取首个 Location 的行号,读 content 对应行。
//   - 行号越界(防御性)→ 不抑制。
//   - 检查两处:行首(忽略前导空白,行首出现否定词=整句否定)
//     或匹配点前 lookBehindChars 字符(就近否定)。
//   - 任一命中 → 抑制。
//
// 仅用于 content 字段匹配。drop pre-emit,不进处置生命周期。
func IsNegatedByContext(content string, locs []Location) bool {
	if len(locs) == 0 {
		return false // command 命中不抑制
	}
	lines := strings.Split(content, "\n")
	lineIdx := locs[0].Line - 1 // Location.Line 是 1-based
	if lineIdx < 0 || lineIdx >= len(lines) {
		return false // 行号越界,不误删
	}
	line := lines[lineIdx]

	// 1. 行首(忽略前导空白)有否定词 → 整句否定
	trimmed := strings.TrimLeft(line, " \t")
	lower := strings.ToLower(trimmed)
	for _, w := range negationWords {
		if strings.HasPrefix(lower, strings.ToLower(w)) {
			return true
		}
	}

	// 2. 匹配点前 lookBehindChars 字符内有否定词 → 就近否定
	col := locs[0].StartCol // 1-based
	if col > 1 {
		start := col - 1 - lookBehindChars
		if start < 0 {
			start = 0
		}
		before := strings.ToLower(line[start : col-1])
		for _, w := range negationWords {
			if strings.Contains(before, strings.ToLower(w)) {
				return true
			}
		}
	}
	return false
}
