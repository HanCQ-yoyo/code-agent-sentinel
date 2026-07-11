package ruleengine

import (
	"encoding/base64"
	"regexp"
	"strings"
)

// Deobfuscate 返回原始文本 + 各反混淆变体。
// 原始文本始终是 out[0],后续元素按 methods 顺序追加(不链式,每种独立)。
// 方法:zero_width / html_comment / base64 / leetspeak。
func Deobfuscate(text string, methods []string) []string {
	out := []string{text}
	for _, m := range methods {
		switch m {
		case "zero_width":
			out = append(out, stripZeroWidth(text))
		case "html_comment":
			out = append(out, stripHTMLComments(text))
		case "base64":
			out = append(out, decodeBase64Chunks(text)...)
		case "leetspeak":
			out = append(out, deleet(text))
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

var b64Re = regexp.MustCompile(`[A-Za-z0-9+/=]{40,}`)
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
	_ = b64Re
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
