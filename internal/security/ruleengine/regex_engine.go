package ruleengine

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

// CompiledRegex 统一 RE2 与 regexp2 的匹配接口。
// 标准库 *regexp.Regexp 的原生签名是 FindAllStringIndex(string, int) [][]int,
// 与本接口不兼容(参数数与元素类型 [2]int vs []int 均不同),故需 re2Wrapper 包装。
// regexp2 经 regexp2Wrapper 适配。firstNonExcludedHit / applyPostExclude /
// lookupRegex 均面向此接口,规则引擎的其余部分无需感知底层是 RE2 还是 regexp2。
type CompiledRegex interface {
	FindAllStringIndex(s string) [][2]int
	MatchString(s string) bool
	FindString(s string) string
}

// needsBacktracking 检测模式是否含 RE2 不支持的 lookahead/lookbehind。
// RE2 不支持 (?= / (?! / (?<= / (?<!。
// dcg 的 safe_pattern 与 flag 遍历规则大量使用这些构造(~737 条规则),
// 检测到任一即返回 true,CompilePattern 会改走 regexp2 编译。
// 注意:仅在模式体本身检测,不误报 (?P<name>(命名捕获)等合法 RE2 构造。
func needsBacktracking(pattern string) bool {
	return strings.Contains(pattern, "(?=") ||
		strings.Contains(pattern, "(?!") ||
		strings.Contains(pattern, "(?<=") ||
		strings.Contains(pattern, "(?<!")
}

// CompilePattern 替换原 compileRegexPattern:含 lookahead/lookbehind 走 regexp2,
// 其余走 RE2(标准库 regexp)。dotall 透传:
//   - RE2:在 (?i)(?m) 前缀后追加 (?s)(与原 compileRegexPattern 行为一致);
//   - regexp2:在 IgnoreCase|Multiline 基础上叠加 Singleline flag(让 . 匹配换行)。
//
// 调用方应统一使用此函数或 compileRegexPattern(薄包装)编译规则正则,
// 不再直接 regexp.Compile,以保证 lookahead/lookbehind 规则可编译。
func CompilePattern(pattern string, dotall bool) (CompiledRegex, error) {
	if needsBacktracking(pattern) {
		return compileRegexp2(pattern, dotall)
	}
	return compileRE2(pattern, dotall)
}

// compileRE2 走标准库 regexp,注入 (?i)(?m)(?s?) 前缀(与原 compileRegexPattern 一致)。
// 用 re2Wrapper 包装以满足 CompiledRegex 接口(标准库 FindAllStringIndex 签名不同)。
func compileRE2(pattern string, dotall bool) (CompiledRegex, error) {
	full := "(?i)(?m)"
	if dotall {
		full += "(?s)"
	}
	full += pattern
	re, err := regexp.Compile(full)
	if err != nil {
		return nil, err
	}
	return &re2Wrapper{re: re}, nil
}

// re2Wrapper 适配标准库 *regexp.Regexp 到 CompiledRegex 接口。
// 标准库 FindAllStringIndex(s, n) [][]int → 本接口 FindAllStringIndex(s) [][2]int。
// n=-1 取全部匹配;每个 []int(长度 2)转 [2]int。
type re2Wrapper struct {
	re *regexp.Regexp
}

func (w *re2Wrapper) FindAllStringIndex(s string) [][2]int {
	matches := w.re.FindAllStringIndex(s, -1)
	out := make([][2]int, len(matches))
	for i, m := range matches {
		out[i] = [2]int{m[0], m[1]}
	}
	return out
}

func (w *re2Wrapper) MatchString(s string) bool {
	return w.re.MatchString(s)
}

func (w *re2Wrapper) FindString(s string) string {
	return w.re.FindString(s)
}

// compileRegexp2 走 github.com/dlclark/regexp2,支持 lookahead/lookbehind。
// flags 对齐 RE2 行为:
//   - IgnoreCase 对应 RE2 的 (?i);
//   - Multiline 对应 (?m)(^/$ 按 \n 锚定);
//   - dotall=true 时叠加 Singleline(对应 (?s),让 . 匹配换行)。
//
// 注意:regexp2 不接受 (?i)(?m)(?s) 前缀(RE2 语法),故通过 flag 传入。
func compileRegexp2(pattern string, dotall bool) (CompiledRegex, error) {
	var flags regexp2.RegexOptions = regexp2.IgnoreCase | regexp2.Multiline
	if dotall {
		flags |= regexp2.Singleline
	}
	re, err := regexp2.Compile(pattern, flags)
	if err != nil {
		return nil, err
	}
	return &regexp2Wrapper{re: re}, nil
}

// regexp2Wrapper 适配 dlclark/regexp2.Regexp 到 CompiledRegex 接口。
//
// regexp2 用 FindStringMatch 返回 *Match(基于 rune 偏移),需转换为字节偏移
// 以满足 FindAllStringIndex 的 [][2]int 契约(与 RE2 一致)。
//
// 实现:rune 偏移 → 字节偏移用 text[:runeStart] 逐 rune 推进。
// 这比 brief 提的 strings.Index(s, matchText) 更准:
//   - strings.Index 会在重复匹配时定位到错误出现(如 "ab ab" 第二个 "ab" 仍返回 0);
//   - 本实现用 Match.Index(rune 偏移)逐次推进,精确反映 regexp2 的匹配位置。
//
// 性能:对长文本 O(n×runeCount),但 ruleengine 正则作用于单条资产文本,
// 不是热点路径,可接受;若 P2/P4 出现性能问题再优化。
type regexp2Wrapper struct {
	re *regexp2.Regexp
}

// runeOffsetToByte 把 rune 偏移(runeIdx,基于 regexp2 的 Match.Index)转字节偏移。
// text 为原始字符串。越界时返回 len(text)(容错,匹配末尾)。
func runeOffsetToByte(text string, runeIdx int) int {
	if runeIdx <= 0 {
		return 0
	}
	byteOff := 0
	for i := 0; i < runeIdx; i++ {
		// 按 UTF-8 编码宽度推进一个 rune
		_, sz := utf8.DecodeRuneInString(text[byteOff:])
		if sz == 0 {
			return len(text)
		}
		byteOff += sz
		if byteOff >= len(text) {
			return len(text)
		}
	}
	return byteOff
}

// FindAllStringIndex 返回所有非重叠匹配的字节区间 [start,end)。
// 通过反复调用 FindNextMatch 迭代所有匹配;每个 Match.Index 是 rune 偏移,
// 用 runeOffsetToByte 转字节偏移;Match.Length 是 rune 计数,转字节长度
// 用匹配文本的字节长度(m.String() 后取 len)。
func (w *regexp2Wrapper) FindAllStringIndex(s string) [][2]int {
	var out [][2]int
	m, err := w.re.FindStringMatch(s)
	if err != nil {
		return out
	}
	for m != nil {
		start := runeOffsetToByte(s, m.Index)
		matchText := m.String()
		end := start + len(matchText)
		if start < 0 || start > len(s) || end > len(s) {
			// 容错:rune→字节转换异常时跳过此匹配,不破坏循环
			m, err = w.re.FindNextMatch(m)
			if err != nil {
				break
			}
			continue
		}
		out = append(out, [2]int{start, end})
		m, err = w.re.FindNextMatch(m)
		if err != nil {
			break
		}
	}
	return out
}

// MatchString 返回是否存在匹配(忽略 regexp2 的 error,仅判 m != nil)。
// regexp2 的 error 在正常输入下罕见(主要是 pattern 编译错误,已在 Compile 时校验);
// 运行期 error 视为无匹配,与 RE2 的 MatchString 行为对齐。
func (w *regexp2Wrapper) MatchString(s string) bool {
	m, _ := w.re.FindStringMatch(s)
	return m != nil
}

// FindString 返回首个匹配的字符串,无匹配返回 ""。
func (w *regexp2Wrapper) FindString(s string) string {
	m, _ := w.re.FindStringMatch(s)
	if m == nil {
		return ""
	}
	return m.String()
}
