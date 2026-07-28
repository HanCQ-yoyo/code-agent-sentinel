package ruleengine

import (
	"strings"
)

// CollectKeywords 收集所有规则的 metadata.keywords(每域手工声明的元数据),去重保序。
// 只收集 pack 的 keywords 字段,不从正则提取。关键词是手工声明的元数据,与 lookahead 正则正交。
//
// YAML 解码后 Rule.Metadata 是 map[string]any,metadata.keywords 的 flow-list
// [rm, shred] 解码为 []any(每元素 any 持 string)。本函数按此类型断言。
func CollectKeywords(rules []Rule) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range rules {
		v, ok := r.Metadata["keywords"]
		if !ok {
			continue
		}
		list, ok := v.([]any)
		if !ok {
			continue
		}
		for _, item := range list {
			kw, ok := item.(string)
			if !ok || kw == "" || seen[kw] {
				continue
			}
			seen[kw] = true
			out = append(out, kw)
		}
	}
	return out
}

// QuickReject 快速放行判断。
// 返回 true = 放行(跳过昂贵的 pack 正则精检),false = 进入精检。
// 规则:
//   - 空 keywords 保守返回 false(防漏放行)。
//   - 命中任一关键词(词边界)→ false(进入精检)。
//   - 未命中且无混淆字符(\ ' ")→ true(放行)。
//   - 未命中但有混淆字符 → false(回退 normalize 重判,防 g\it 漏报)。
//
// 语义:**命中关键词=进入精检(false);未命中=放行(true)**。
// 多词关键词(含空白)走 matchMultiwordKeyword(容忍任意空白量)。
func QuickReject(cmd string, keywords []string) bool {
	if len(keywords) == 0 {
		return false // 保守:空列表不 reject
	}
	bytes := []byte(cmd)
	anyHit := false
	for _, kw := range keywords {
		if strings.ContainsAny(kw, " \t") {
			// 多词关键词:容忍任意空白量
			if matchMultiwordKeyword(cmd, kw) {
				anyHit = true
				break
			}
			continue
		}
		if containsWord(bytes, []byte(kw)) {
			anyHit = true
			break
		}
	}
	if anyHit {
		return false // 命中 → 进入精检
	}
	// 未命中:检查混淆字符
	for _, b := range bytes {
		if b == '\\' || b == '\'' || b == '"' {
			return false // 有混淆 → 回退 normalize 重判
		}
	}
	return true // 无混淆 → 放行
}

// containsWord 词边界子串匹配。
// is_word_byte = alnum | '_'。词边界:匹配前后字节非 word byte(或串首/串尾)。
// 大小写不敏感(ASCII fold),保证 GIT/git/Git 均命中关键词 git。
func containsWord(haystack, needle []byte) bool {
	n := len(needle)
	if n == 0 {
		return false
	}
	for i := 0; i+n <= len(haystack); i++ {
		if !equalFold(haystack[i:i+n], needle) {
			continue
		}
		before := i == 0 || !isWordByte(haystack[i-1])
		after := i+n == len(haystack) || !isWordByte(haystack[i+n])
		if before && after {
			return true
		}
	}
	return false
}

// isWordByte 判断字节是否为 word 字符(alnum | '_')。
// 非词字节(. / - 空格等)构成词边界。
func isWordByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// equalFold ASCII 大小写不敏感比较(比 bytes.EqualFold 更窄:仅 ASCII)。
// 关键词匹配只关心 ASCII 命令名,非 ASCII 字节直接不等。
func equalFold(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if toLower(a[i]) != toLower(b[i]) {
			return false
		}
	}
	return true
}

// toLower ASCII 小写转换。
func toLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

// matchMultiwordKeyword 多词关键词匹配(容忍任意空白,如 "git push" 匹配 "git   push")。
// 顺序匹配各词,中间允许任意空白。
// 大小写不敏感(ToLower 比较)。
func matchMultiwordKeyword(cmd, kw string) bool {
	words := strings.Fields(kw)
	if len(words) == 0 {
		return false
	}
	lowCmd := strings.ToLower(cmd)
	idx := 0
	for _, w := range words {
		j := strings.Index(lowCmd[idx:], strings.ToLower(w))
		if j < 0 {
			return false
		}
		idx += j + len(w)
	}
	return true
}
