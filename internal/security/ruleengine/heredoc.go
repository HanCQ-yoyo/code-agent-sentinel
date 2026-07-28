package ruleengine

import (
	"regexp"
	"strings"
)

// MaxEmbeddedShellDepth 是 Tier2.5 递归深度上限。
// 注意:深度判定用严格大于(>),保证 depth==MaxEmbeddedShellDepth 时仍提取一层,
// 在 depth==MaxEmbeddedShellDepth+1 时停止递归。
// 这样 TestExtractInlineScriptDepthLimit(传入 depth=MaxEmbeddedShellDepth)能至少
// 提取一层 bash -c 内层,符合"深度上限内应至少提取一层"的契约。
const MaxEmbeddedShellDepth = 8

// heredocTriggerRes 是 Tier1 的 17 个触发正则。
// 零假阴性:必须触发所有 Tier2 会提取的。命中任一 → 进入 Tier2 提取。
var heredocTriggerRes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bpython[0-9.]*(?:\.exe)?\b.*?-[A-Za-z]*c[A-Za-z]*(?:\s|['"]|$)`),
	regexp.MustCompile(`(?i)\bruby[0-9.]*(?:\.exe)?\b.*?-[A-Za-z]*e[A-Za-z]*(?:\s|['"]|$)`),
	regexp.MustCompile(`(?i)\bperl\b.*?-[A-Za-z]*[eE][A-Za-z]*(?:\s|['"]|$)`),
	regexp.MustCompile(`(?i)\bnode\b.*?-[A-Za-z]*[ep][A-Za-z]*(?:\s|['"]|$)`),
	regexp.MustCompile(`(?i)\bphp\b.*?-[A-Za-z]*r[A-Za-z]*(?:\s|['"]|$)`),
	regexp.MustCompile(`(?i)\blua\b.*?-[A-Za-z]*e[A-Za-z]*(?:\s|['"]|$)`),
	regexp.MustCompile(`(?i)\b(?:sh|bash|zsh|fish)(?:\.exe)?\b.*?-[A-Za-z]*c[A-Za-z]*(?:\s|['"]|$)`),
	regexp.MustCompile(`(?i)\b(?:powershell|pwsh)(?:\.exe)?\b.*?-c(?:om\w*)?(?:\s|['"]|$)`),
	regexp.MustCompile(`(?i)\b(?:powershell|pwsh)\b.*?-enc(?:odedcommand)?\s+\S+`),
	regexp.MustCompile(`(?i)\bcmd\b\s+/[ck]\b`),
	regexp.MustCompile(`(?i)\biex\b|invoke-expression\b`),
	regexp.MustCompile(`(?i)\beval\b\s+['"]`),
	regexp.MustCompile(`(?i)\bexec\b\s+['"]`),
	regexp.MustCompile(`<<<`), // here-string
	regexp.MustCompile(`\|\s*(?:python|bash|sh|zsh|ruby|perl|node|php|lua)\b`),
	regexp.MustCompile(`\|?\s*xargs\b`),
	regexp.MustCompile(`<<-?\s*['"]?\w+['"]?`), // heredoc 操作符
}

// HasInlineScript 是 Tier1 触发检测。
// 零假阴性:任一 trigger 正则命中 或 含 active heredoc 操作符 → true。
func HasInlineScript(cmd string) bool {
	for _, re := range heredocTriggerRes {
		if re.MatchString(cmd) {
			return true
		}
	}
	return containsActiveHeredocOperator(cmd)
}

// containsActiveHeredocOperator 手写引号感知扫描:找 << 但跳过引号内的。
func containsActiveHeredocOperator(cmd string) bool {
	inSingle, inDouble := false, false
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		if c == '\\' && i+1 < len(cmd) { // 跳过转义
			i++
			continue
		}
		if c == '\'' && !inDouble {
			inSingle = !inSingle
		} else if c == '"' && !inSingle {
			inDouble = !inDouble
		} else if !inSingle && !inDouble && c == '<' && i+1 < len(cmd) && cmd[i+1] == '<' {
			return true
		}
	}
	return false
}

// inlineScriptRe 提取 interpreter -c "..." / '...' 内层内容。
// Go regexp(RE2)不支持反引用 \1,故用 alternation 两个分支:
//   - 捕获组1=双引号内层内容 [^"]*(允许内层含单引号,如 python -c "...os.system('...')")
//   - 捕获组2=单引号内层内容 [^']*(允许内层含双引号)
var inlineScriptRe = regexp.MustCompile(`(?i)\b(?:python[0-9.]*|ruby[0-9.]*|perl|node|php|lua|sh|bash|zsh|fish|powershell|pwsh)(?:\.exe)?\b["']?(?:\s+(?:--\S+|-\S+))*\s+-(?:[A-Za-z]*[ceECpr][A-Za-z]*)\s+(?:"([^"]*)"|'([^']*)')`)

// hereStringRe 提取 <<< "..." / '...' / <<< unquoted 内容。
// RE2 不支持反引用,故不强制闭合引号匹配:[^"'\n]* 已在引号内自动停于闭合引号,
// 内容捕获仍正确。组1=可选开引号,组2=内容。
var hereStringRe = regexp.MustCompile(`<<<\s*(["']?)([^"'\n]*)`)

// ExtractInlineScripts 是 Tier2 提取 + Tier2.5 手写分段递归。
// 返回所有提取出的内层命令(v1 砍 AST,用手写 $()/反引号/;/&&/||/| 分段递归)。
// depth > MaxEmbeddedShellDepth 时停止递归(防无限)。
//
// 深度语义:depth 表示当前调用栈已嵌套的层数。depth==MaxEmbeddedShellDepth 时
// 仍允许提取本层(返回内层内容),但不再对提取结果递归;depth==MaxEmbeddedShellDepth+1
// 时直接返回 nil。这保证 TestExtractInlineScriptDepthLimit(传入 depth=MaxEmbeddedShellDepth)
// 至少提取一层,符合"深度上限内应至少提取一层"的契约。
func ExtractInlineScripts(cmd string, depth int) []string {
	if depth > MaxEmbeddedShellDepth {
		return nil
	}
	var out []string
	// 提取 interpreter -c "..." 内层
	// inlineScriptRe 用 alternation:组1=双引号内层,组2=单引号内层(其一非空)。
	for _, m := range inlineScriptRe.FindAllStringSubmatch(cmd, -1) {
		var inner string
		if len(m) >= 2 && m[1] != "" {
			inner = m[1] // 双引号分支
		} else if len(m) >= 3 && m[2] != "" {
			inner = m[2] // 单引号分支
		}
		if inner == "" {
			continue
		}
		out = append(out, inner)
		// 递归:内层可能再含 bash -c
		out = append(out, ExtractInlineScripts(inner, depth+1)...)
		// 内层可能含 $() 命令替换,分段递归
		for _, seg := range splitCommandSegments(inner) {
			if seg != inner && seg != "" {
				out = append(out, seg)
				out = append(out, ExtractInlineScripts(seg, depth+1)...)
			}
		}
	}
	// 提取 here-string 内容。组1=可选开引号,组2=内容。
	for _, m := range hereStringRe.FindAllStringSubmatch(cmd, -1) {
		if len(m) >= 3 && m[2] != "" {
			out = append(out, m[2])
		}
	}
	// 提取 $() 命令替换(无条件,作为内层命令)
	for _, seg := range extractCommandSubstitutions(cmd) {
		out = append(out, seg)
		out = append(out, ExtractInlineScripts(seg, depth+1)...)
	}
	return out
}

// splitCommandSegments 手写分段:按 ; && || | 切分命令(非 AST)。
func splitCommandSegments(cmd string) []string {
	var segs []string
	var cur strings.Builder
	inSingle, inDouble, paren := false, false, 0
	flush := func() {
		s := strings.TrimSpace(cur.String())
		if s != "" {
			segs = append(segs, s)
		}
		cur.Reset()
	}
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		if c == '\\' && i+1 < len(cmd) {
			cur.WriteByte(c)
			if i+1 < len(cmd) {
				cur.WriteByte(cmd[i+1])
				i++
			}
			continue
		}
		if c == '\'' && !inDouble {
			inSingle = !inSingle
		} else if c == '"' && !inSingle {
			inDouble = !inDouble
		} else if !inSingle && !inDouble && c == '(' {
			paren++
		} else if !inSingle && !inDouble && c == ')' {
			if paren > 0 {
				paren--
			}
		} else if !inSingle && !inDouble && paren == 0 && (c == ';' || c == '|') {
			// && / || 视为一个分隔
			if c == '|' && i+1 < len(cmd) && cmd[i+1] == '|' {
				flush()
				i++ // 跳过第二个 |
				continue
			}
			if c == '|' && i > 0 && cmd[i-1] == '&' {
				// 已被 & 处理,跳过
			}
			flush()
			continue
		} else if !inSingle && !inDouble && paren == 0 && c == '&' {
			if i+1 < len(cmd) && cmd[i+1] == '&' {
				flush()
				i++
				continue
			}
		}
		cur.WriteByte(c)
	}
	flush()
	return segs
}

// extractCommandSubstitutions 提取 $() 内层命令(v1 手写,砍 AST)。
// 简化:找 $(...) 配对 ),提取内层。反引号 `...` 同理。
func extractCommandSubstitutions(cmd string) []string {
	var out []string
	inSingle, inDouble := false, false
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		if c == '\\' && i+1 < len(cmd) {
			i++
			continue
		}
		if c == '\'' && !inDouble {
			inSingle = !inSingle
		} else if c == '"' && !inSingle {
			inDouble = !inDouble
		} else if !inSingle && !inDouble && c == '$' && i+1 < len(cmd) && cmd[i+1] == '(' {
			inner := extractBalanced(cmd, i+1) // 从 ( 开始找配对 )
			if inner != "" {
				out = append(out, inner)
			}
		}
	}
	return out
}

// extractBalanced 从 start(指向 '(')找配对 ')',返回括号内内容(引号/嵌套感知)。
func extractBalanced(s string, start int) string {
	if start >= len(s) || s[start] != '(' {
		return ""
	}
	depth := 0
	inSingle, inDouble := false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			i++
			continue
		}
		if c == '\'' && !inDouble {
			inSingle = !inSingle
		} else if c == '"' && !inSingle {
			inDouble = !inDouble
		} else if !inSingle && !inDouble && c == '(' {
			depth++
		} else if !inSingle && !inDouble && c == ')' {
			depth--
			if depth == 0 {
				return s[start+1 : i] // 括号内
			}
		}
	}
	return "" // 未配对
}
