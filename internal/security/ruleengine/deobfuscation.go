package ruleengine

import (
	"encoding/base64"
	"regexp"
	"strings"
)

// Candidate 是一个反混淆候选变体。
// Method 是产生该变体的反混淆方法名(空串=原始文本本身)。
// Text 是反混淆后的文本。
type Candidate struct {
	Method string
	Text   string
}

// Deobfuscate 返回原始文本 + 各反混淆变体。
// out[0] 始终是原始文本(Method=""),后续按 methods 顺序追加(不链式,每种独立)。
// base64 方法可能产生多个变体(文本中每个可解码块各一个),所以返回长度可大于 1+len(methods)。
// 方法:zero_width / html_comment / base64 / leetspeak / wrapper_strip / ansi_c_decode。
func Deobfuscate(text string, methods []string) []Candidate {
	out := []Candidate{{Method: "", Text: text}}
	for _, m := range methods {
		switch m {
		case "zero_width":
			out = append(out, Candidate{Method: m, Text: stripZeroWidth(text)})
		case "html_comment":
			out = append(out, Candidate{Method: m, Text: stripHTMLComments(text)})
		case "base64":
			for _, dec := range decodeBase64Chunks(text) {
				out = append(out, Candidate{Method: m, Text: dec})
			}
		case "leetspeak":
			out = append(out, Candidate{Method: m, Text: deleet(text)})
		case "wrapper_strip":
			out = append(out, Candidate{Method: m, Text: stripWrappers(text)})
		case "ansi_c_decode":
			out = append(out, Candidate{Method: m, Text: decodeAnsiC(text)})
		}
	}
	return out
}

func stripZeroWidth(s string) string {
	// 用 \u 转义而非字面量:Go 编译器拒绝文件中间的 BOM(U+FEFF)字面量。
	var b strings.Builder
	for _, r := range s {
		if r == '\u200b' || r == '\u200c' || r == '\u200d' || r == '\ufeff' || r == '\u2060' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

var htmlCommentRe = regexp.MustCompile(`<!--[\s\S]*?-->`)

func stripHTMLComments(s string) string {
	return htmlCommentRe.ReplaceAllString(s, "")
}

var shortB64Re = regexp.MustCompile(`[A-Za-z0-9+/=]{16,}`)

// decodeBase64Chunks 尝试解码文本里的 base64 片段,返回解码后的字符串。
func decodeBase64Chunks(s string) []string {
	var out []string
	re := shortB64Re
	for _, m := range re.FindAllString(s, -1) {
		if b, err := base64.StdEncoding.DecodeString(m); err == nil {
			if isPrintable(b) {
				out = append(out, string(b))
			}
		}
	}
	return out
}

func deleet(s string) string {
	r := strings.NewReplacer("0", "o", "1", "i", "3", "e", "4", "a", "5", "s", "7", "t", "@", "a", "$", "s")
	return r.Replace(s)
}

func isPrintable(b []byte) bool {
	for _, c := range b {
		if c < 9 || (c > 13 && c < 32) {
			return false
		}
	}
	return len(b) > 0
}

// stripWrappers 剥离命令 wrapper:sudo / env VAR=... / command / 反斜杠续行。
// 对应 dcg normalize 的 wrapper 剥离(简化版:前缀 wrapper + 续行折叠)。
var (
	sudoPrefixRe    = regexp.MustCompile(`(?i)^\s*sudo(?:\s+-\S+)*\s+`)
	envPrefixRe     = regexp.MustCompile(`(?i)^\s*env(?:\s+[A-Za-z_]\w*=\S+)+\s+`)
	commandPrefixRe = regexp.MustCompile(`(?i)^\s*command\s+`)
	backslashRe     = regexp.MustCompile(`\\\s*\n\s*`)
)

func stripWrappers(s string) string {
	s = backslashRe.ReplaceAllString(s, " ")
	for {
		next := sudoPrefixRe.ReplaceAllString(s, "")
		next = envPrefixRe.ReplaceAllString(next, "")
		next = commandPrefixRe.ReplaceAllString(next, "")
		if next == s {
			break
		}
		s = next
	}
	return strings.TrimSpace(s)
}

// ansiCRe 匹配 $'...' ANSI-C 引号串。
var ansiCRe = regexp.MustCompile(`\$'([^']*)'`)

// decodeAnsiC 解码 Bash ANSI-C 引号:$'\x72\x6d' → rm。
// 支持 \xNN 十六进制、\t\r\n 等常见转义。
func decodeAnsiC(s string) string {
	return ansiCRe.ReplaceAllStringFunc(s, func(m string) string {
		// m 形如 $'...\x72...'
		inner := m[2 : len(m)-1] // 去掉 $' 和 '
		return decodeAnsiCEscapes(inner)
	})
}

// decodeAnsiCEscapes 处理 \xNN / \t \r \n \\ \' 等。
func decodeAnsiCEscapes(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'x':
			if i+2 < len(s) {
				if v, ok := parseHex(s[i+1], s[i+2]); ok {
					b.WriteByte(v)
					i += 2
					continue
				}
			}
			b.WriteByte('x')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'n':
			b.WriteByte('\n')
		case '\\', '\'':
			b.WriteByte(s[i])
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func parseHex(hi, lo byte) (byte, bool) {
	h := hexVal(hi)
	l := hexVal(lo)
	if h < 0 || l < 0 {
		return 0, false
	}
	return byte(h*16 + l), true
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}
