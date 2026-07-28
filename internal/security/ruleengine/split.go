package ruleengine

import "strings"

// SplitCommand 按 &&/;/||/| 拆分命令为片段(引号/$()/()/反引号感知)。
// 替换 R2 heredoc.go 的 splitCommandSegments(粗版,&& 边界 bug,spec §3.2)。
// 规则:
//   - 分隔符仅在顶层 executed 区生效:引号内、$() 内、() 内、反引号内不拆。
//   - && / || / ; / | 均为分隔符(管道 | 后段独立执行,也拆)。
//   - 单片段(无分隔符)返回 []string{cmd}。
//   - panic 兜底:返回 []string{cmd}(整条当单片段,不因拆分失败漏报)。
//
// 与 span.go ClassifySpans 同源的 panic 兜底模式:用命名返回值 segs,
// 在 deferred recover 中赋值 fallback,确保 panic 时返回值被真正改写
// (Go 的 defer recover 无法修改非命名返回值——Task 1 review 曾捕获此 bug)。
func SplitCommand(cmd string) (segs []string) {
	defer func() {
		if r := recover(); r != nil {
			// panic 兜底:整条当单片段,不因拆分失败漏报(fail-open 铁律)
			segs = []string{cmd}
		}
	}()
	segs = splitImplFn(cmd)
	if len(segs) == 0 {
		// 空输入或实现返回空:同样兜底单片段(与 splitCommandSegments 行为对齐,
		// 避免下游 range nil 漏报;空 cmd 仍当单片段)
		return []string{cmd}
	}
	return segs
}

// splitImplFn 是实现钩子(包级变量,测试可注入 panic),与 span.go classifySpansImplFn 同模式。
var splitImplFn = splitImpl

// splitImpl 是 SplitCommand 的实现(无 panic 兜底,由 SplitCommand 包裹)。
// 复用 span.go 的 extractSubstBody 跳过 $() 内层(引号/嵌套感知)。
func splitImpl(cmd string) []string {
	var segs []string
	var cur strings.Builder
	inSingle, inDouble := false, false
	paren := 0 // 顶层 () 深度(非 $(),由 extractSubstBody 处理)
	flush := func() {
		s := strings.TrimSpace(cur.String())
		if s != "" {
			segs = append(segs, s)
		}
		cur.Reset()
	}
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		// 转义符:保留两字节(引号内外都保留,与 span.go 一致)
		if c == '\\' && i+1 < len(cmd) {
			cur.WriteByte(c)
			cur.WriteByte(cmd[i+1])
			i++
			continue
		}
		if c == '\'' && !inDouble {
			inSingle = !inSingle
			cur.WriteByte(c)
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			cur.WriteByte(c)
			continue
		}
		// $() 配对:进入时不拆,整体写入 cur 并跳过(复用 span.go extractSubstBody)
		if !inSingle && !inDouble && c == '$' && i+1 < len(cmd) && cmd[i+1] == '(' {
			inner, ok := extractSubstBody(cmd, i+1) // 复用 span.go(引号/嵌套感知)
			if ok {
				// 写回 $(inner):2 字节前缀 + inner + 1 字节后缀
				cur.WriteString(cmd[i : i+2+len(inner)+1])
				i += 2 + len(inner) // 跳过 $( + inner + ) 共 2+len(inner)+1 字节
				continue
			}
			// 未配对:$ 原样写入,后续按普通字符处理
		}
		// 反引号配对:到下一个反引号为止(简化,引号内反引号无意义)
		if !inSingle && !inDouble && c == '`' {
			cur.WriteByte(c)
			for i+1 < len(cmd) && cmd[i+1] != '`' {
				i++
				cur.WriteByte(cmd[i])
			}
			if i+1 < len(cmd) {
				cur.WriteByte(cmd[i+1]) // 闭合反引号
				i++
			}
			continue
		}
		// () 配对(非 $(),已由 extractSubstBody 处理):顶层 () 内不拆
		if !inSingle && !inDouble && c == '(' {
			paren++
			cur.WriteByte(c)
			continue
		}
		if !inSingle && !inDouble && c == ')' {
			if paren > 0 {
				paren--
			}
			cur.WriteByte(c)
			continue
		}
		// 顶层分隔符(引号/$()/()/反引号 外):&& / || / | / ;
		// R2 粗版 splitCommandSegments 的 && 边界 bug 在此修干净:
		//   - && 先于单 & 判定,flush 后 i++ 跳过第二个 &;
		//   - || 同理先于单 | 判定;
		//   - 单 | 是管道(后段独立执行),也 flush。
		if !inSingle && !inDouble && paren == 0 {
			if c == '&' && i+1 < len(cmd) && cmd[i+1] == '&' {
				flush()
				i++ // 跳过第二个 &
				continue
			}
			if c == '|' && i+1 < len(cmd) && cmd[i+1] == '|' {
				flush()
				i++ // 跳过第二个 |
				continue
			}
			if c == '|' {
				flush() // 管道:后段独立执行,也拆
				continue
			}
			if c == ';' {
				flush()
				continue
			}
		}
		cur.WriteByte(c)
	}
	flush()
	return segs
}
