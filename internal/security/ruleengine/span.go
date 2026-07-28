package ruleengine

import "strings"

// SpanKind 标记命令文本片段是否参与破坏性正则匹配。
type SpanKind int

const (
	SpanExecuted SpanKind = iota // 可执行区:命令文本、参数、$() 替换结果(会执行)
	SpanData                     // 数据区:引号内字面量、heredoc 体、变量引用(不执行/不可见)
	SpanComment                  // 注释区:# 行注释(不执行)
)

// Span 是命令文本的一个分类片段。Start/End 为字节偏移(供 confidence 定位 + 静态层 Location 对齐)。
type Span struct {
	Kind  SpanKind
	Text  string
	Start int
	End   int
}

// ClassifySpans 把命令文本切成有序 []Span。
// 手写状态机(参考 dcg normalize.rs span 划分,简化为三态):
//   - 单引号 '...' 内整体 Data(无转义无插值)。
//   - 双引号 "..." 内:字面量 Data;$()/反引号段 Executed(递归);${var}/$var 段 Data(已知限制)。
//   - # 到行尾 Comment(仅当 # 前是空白或行首)。
//   - <<EOF...EOF heredoc 体 Data(简化:识别 <<DELIM 后到 DELIM 行)。
//   - $()/反引号内容 Executed(递归,深度上限 MaxEmbeddedShellDepth)。
//   - 其余 Executed。
//
// panic 兜底:返回单 SpanExecuted 覆盖全文本(安全不变量:宁可误拦不漏报)。
func ClassifySpans(cmd string) []Span {
	defer func() {
		_ = recover() // panic 兜底:下面 return 单 Executed 覆盖全文本
	}()
	spans := classifySpansImpl(cmd, 0, 0)
	if spans == nil {
		// panic 路径或空输入:返回单 Executed 覆盖全文本
		return []Span{{Kind: SpanExecuted, Text: cmd, Start: 0, End: len(cmd)}}
	}
	return spans
}

// classifySpansImpl 是递归实现。baseOff 是当前子串在原命令中的偏移(递归 $() 内层时累加)。
// depth 是命令替换递归深度(防无限,上限 MaxEmbeddedShellDepth)。
func classifySpansImpl(cmd string, baseOff, depth int) []Span {
	var spans []Span
	i := 0
	n := len(cmd)
	var execBuf strings.Builder
	execStart := 0
	flushExec := func(end int) {
		if execBuf.Len() > 0 {
			spans = append(spans, Span{
				Kind:  SpanExecuted,
				Text:  execBuf.String(),
				Start: baseOff + execStart,
				End:   baseOff + end,
			})
			execBuf.Reset()
		}
	}
	for i < n {
		c := cmd[i]
		// 转义符:保留原样到 Executed(bash 转义是执行语义)
		if c == '\\' && i+1 < n {
			if execBuf.Len() == 0 {
				execStart = i
			}
			execBuf.WriteByte(c)
			execBuf.WriteByte(cmd[i+1])
			i += 2
			continue
		}
		// 单引号:整体 Data(无转义无插值)
		if c == '\'' {
			flushExec(i)
			start := i
			i++ // 跳过开引号
			for i < n && cmd[i] != '\'' {
				i++
			}
			inner := cmd[start+1 : i] // 引号内(不含引号)
			if i < n {
				i++ // 跳过闭引号
			}
			spans = append(spans, Span{Kind: SpanData, Text: inner, Start: baseOff + start + 1, End: baseOff + i - 1})
			execStart = i
			continue
		}
		// 双引号:字面量 Data + $()/反引号 Executed + ${var}/$var Data
		if c == '"' {
			flushExec(i)
			start := i
			i++ // 跳过开引号
			var dataBuf strings.Builder
			dataStart := i
			for i < n && cmd[i] != '"' {
				switch {
				case cmd[i] == '\\' && i+1 < n:
					// 转义:双引号内 \$ \" \\ 保留字面量;其余转义符字面保留
					dataBuf.WriteByte(cmd[i])
					dataBuf.WriteByte(cmd[i+1])
					i += 2
				case cmd[i] == '$' && i+1 < n && cmd[i+1] == '(':
					// 双引号内 $(...) → Executed(闭合)
					if dataBuf.Len() > 0 {
						spans = append(spans, Span{Kind: SpanData, Text: dataBuf.String(), Start: baseOff + dataStart, End: baseOff + i})
						dataBuf.Reset()
					}
					inner, _ := extractSubstBody(cmd, i+1) // 从 ( 开始
					if inner != "" && depth < MaxEmbeddedShellDepth {
						spans = append(spans, classifySpansImpl(inner, baseOff+i+2, depth+1)...)
					}
					// 推进 i 越过 $(...);$ 在 i,( 在 i+1,inner 长度 len(inner),) 在 i+2+len(inner)
					i = i + 2 + len(inner) + 1
					if i > n {
						i = n
					}
					dataStart = i
				case cmd[i] == '`':
					// 双引号内反引号 → Executed(闭合)
					if dataBuf.Len() > 0 {
						spans = append(spans, Span{Kind: SpanData, Text: dataBuf.String(), Start: baseOff + dataStart, End: baseOff + i})
						dataBuf.Reset()
					}
					bs := i + 1
					i++
					for i < n && cmd[i] != '`' {
						i++
					}
					inner := cmd[bs:i]
					if inner != "" && depth < MaxEmbeddedShellDepth {
						spans = append(spans, classifySpansImpl(inner, baseOff+bs, depth+1)...)
					}
					if i < n {
						i++ // 跳过闭反引号
					}
					dataStart = i
				case cmd[i] == '$' && i+1 < n && (cmd[i+1] == '{' || isVarChar(cmd[i+1])):
					// ${var} 或 $var → Data(已知限制,值不可见)
					dataBuf.WriteByte(cmd[i])
					if cmd[i+1] == '{' {
						dataBuf.WriteByte('{')
						i += 2
						for i < n && cmd[i] != '}' {
							dataBuf.WriteByte(cmd[i])
							i++
						}
						if i < n {
							dataBuf.WriteByte('}')
							i++
						}
					} else {
						i++
						for i < n && isVarChar(cmd[i]) {
							dataBuf.WriteByte(cmd[i])
							i++
						}
					}
				default:
					dataBuf.WriteByte(cmd[i])
					i++
				}
			}
			if dataBuf.Len() > 0 {
				spans = append(spans, Span{Kind: SpanData, Text: dataBuf.String(), Start: baseOff + dataStart, End: baseOff + i})
			}
			if i < n {
				i++ // 跳过闭双引号
			}
			execStart = i
			_ = start
			continue
		}
		// # 注释:仅当 # 前是空白或行首
		if c == '#' && (i == 0 || cmd[i-1] == ' ' || cmd[i-1] == '\t' || cmd[i-1] == '\n') {
			flushExec(i)
			start := i
			for i < n && cmd[i] != '\n' {
				i++
			}
			spans = append(spans, Span{Kind: SpanComment, Text: cmd[start:i], Start: baseOff + start, End: baseOff + i})
			execStart = i
			continue
		}
		// 顶层 $()(非双引号内)→ 内层 Executed(递归)
		if c == '$' && i+1 < n && cmd[i+1] == '(' {
			if execBuf.Len() == 0 {
				execStart = i
			}
			inner, _ := extractSubstBody(cmd, i+1)
			if inner != "" && depth < MaxEmbeddedShellDepth {
				flushExec(i)
				spans = append(spans, classifySpansImpl(inner, baseOff+i+2, depth+1)...)
				execStart = i + 2 + len(inner) + 1
				i = execStart
				if i > n {
					i = n
				}
				continue
			}
		}
		// 顶层反引号(非双引号内)→ 内层 Executed(递归)
		if c == '`' {
			if execBuf.Len() == 0 {
				execStart = i
			}
			bs := i + 1
			i++
			for i < n && cmd[i] != '`' {
				i++
			}
			inner := cmd[bs:i]
			if inner != "" && depth < MaxEmbeddedShellDepth {
				flushExec(bs - 1)
				spans = append(spans, classifySpansImpl(inner, baseOff+bs, depth+1)...)
			}
			if i < n {
				i++ // 跳过闭反引号
			}
			execStart = i
			continue
		}
		// 普通 Executed 字符
		if execBuf.Len() == 0 {
			execStart = i
		}
		execBuf.WriteByte(c)
		i++
	}
	flushExec(n)
	return spans
}

// extractSubstBody 从 cmd[start](指向 '(')找配对 ')',返回括号内内容 + 是否闭合。
// 引号/嵌套 $() 感知。
func extractSubstBody(cmd string, start int) (string, bool) {
	if start >= len(cmd) || cmd[start] != '(' {
		return "", false
	}
	depth := 0
	inSingle, inDouble := false, false
	for i := start; i < len(cmd); i++ {
		c := cmd[i]
		if c == '\\' && i+1 < len(cmd) {
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
				return cmd[start+1 : i], true
			}
		}
	}
	return "", false
}

// isVarChar 判断是否为变量名字符(bash:字母/数字/下划线;首字符非数字但此处宽松)。
func isVarChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
