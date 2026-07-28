package ruleengine

import (
	"regexp"
	"strings"
)

// NormalizeCommand 是运行时拦截的反混淆 normalize(手写状态机)。
// 流水线:剥 wrapper(迭代≤32)→ ANSI-C 解码(仅 executable position)→ 去引号 → 路径展开。
// 与静态层 deobfuscation.go(正则简版)并存:静态层多行 Content 用简版,运行时单命令用本函数。
//
// 不引 shell parser:wrapper 剥离用状态机,ANSI-C 用字符级状态机,
// 路径展开用正则。所有步骤保守:未知选项不剥(防误剥安全命令)。
//
// 变量命名:本文件 regex 变量加 norm 前缀,避免与同包 deobfuscation.go
// (sudoPrefixRe/commandPrefixRe/backslashRe/ansiCRe)冲突——静态层与运行时层
// 各自独立,正则语义不同(静态层简版不剥路径前缀,运行时层剥 /usr/bin/sudo 等)。
const maxWrapperIterations = 32

// NormalizeCommand 返回 normalize 后的命令。
func NormalizeCommand(cmd string) string {
	cmd = stripWrapperPrefixes(cmd)
	cmd = decodeAnsiCExecutable(cmd)
	cmd = dequoteExecutable(cmd)
	cmd = expandPath(cmd)
	return strings.TrimSpace(cmd)
}

var (
	normSudoPrefixRe    = regexp.MustCompile(`(?i)^\s*(?:\S*/)?sudo(?:\s+-[EHnkKSsbiPABuUghpCrDtTt]+(?:\s+\S+)?)*\s+`)
	normEnvAssignRe     = regexp.MustCompile(`(?i)^\s*(?:\S*/)?env(?:\s+-\S+)*(\s+[A-Za-z_]\w*=\S+)+\s+`)
	normEnvFlagOnlyRe   = regexp.MustCompile(`(?i)^\s*(?:\S*/)?env(?:\s+-\S+)+\s+`) // env -i cmd
	normCommandPrefixRe = regexp.MustCompile(`(?i)^\s*(?:\S*/)?command\s+`)
	normExecWrapperRe   = regexp.MustCompile(`(?i)^\s*(?:\S*/)?(?:exec|nohup|time)\s+`)
	normBackslashRe     = regexp.MustCompile(`^\\(\w[\w.-]*)`)
)

// stripWrapperPrefixes 迭代剥 sudo/env/command/exec/nohup/time/反斜杠。
// command -v/-V(query)不剥;纯 env(打印环境)不剥。
func stripWrapperPrefixes(cmd string) string {
	for i := 0; i < maxWrapperIterations; i++ {
		before := cmd
		// command -v/-V 不剥:检查 command 后是否跟 -v/-V
		if m := normCommandPrefixRe.FindString(cmd); m != "" {
			rest := strings.TrimSpace(cmd[len(m):])
			if !strings.HasPrefix(rest, "-v") && !strings.HasPrefix(rest, "-V") {
				cmd = rest
				continue
			}
		}
		// env 纯赋值无命令不剥(打印环境);env -i/-u 等 flag-only + 命令才剥
		if m := normEnvAssignRe.FindString(cmd); m != "" {
			cmd = strings.TrimSpace(cmd[len(m):])
			continue
		}
		if m := normEnvFlagOnlyRe.FindString(cmd); m != "" {
			cmd = strings.TrimSpace(cmd[len(m):])
			continue
		}
		if m := normSudoPrefixRe.FindString(cmd); m != "" {
			cmd = strings.TrimSpace(cmd[len(m):])
			continue
		}
		if m := normExecWrapperRe.FindString(cmd); m != "" {
			cmd = strings.TrimSpace(cmd[len(m):])
			continue
		}
		// normBackslashRe 匹配 ^\word,剥反斜杠保留 word(alias bypass)。
		// m = "\word" 整体,但我们要保留 word → 用捕获组1替换(仅去反斜杠)。
		if m := normBackslashRe.FindStringSubmatchIndex(cmd); m != nil {
			// m[0]:m[1]=整体匹配,m[2]:m[3]=捕获组1(word)
			word := cmd[m[2]:m[3]]
			cmd = word + cmd[m[1]:]
			continue
		}
		if cmd == before {
			break
		}
	}
	return cmd
}

// decodeAnsiCExecutable 解码 $'...' ANSI-C 引号。
// 支持 \xNN \uNNNN \UNNNNNNNN \nnn(八进制)\c \a\b\e\f\n\r\t\v \\ \' \"(由 decodeAnsiCEscapes 处理)。
//
// 数据区不解码原则:运行时只关心被调用的命令名是否被 ANSI-C 混淆,
// 数据区(如 echo 的参数、git commit -m 的消息)中的 $'...' 不解码——避免误判数据区含危险命令。
// 判定:wrapper 剥离后,若命令以 $' 开头(executable 是 ANSI-C 引号串),解码该串;否则不解码。
// 这与静态层 decodeAnsiC(解码所有 $'...')不同:静态层面对多行 Content 需要全扫,运行时层
// 面对单命令只需看 executable position。TestNormalizeDataAreaNotDecoded 验证此行为:
// echo $'rm -rf /' 中 $'...' 在数据区,不解码。
func decodeAnsiCExecutable(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if !strings.HasPrefix(cmd, "$'") {
		return cmd
	}
	// 找匹配的闭合单引号,跳过 \' 转义(v1 简化:不处理 \\' 这种 escaped-backslash + 真引号边界)
	for i := 2; i < len(cmd); i++ {
		if cmd[i] == '\'' && (i == 0 || cmd[i-1] != '\\') {
			inner := cmd[2:i]
			decoded := decodeAnsiCEscapes(inner) // 复用 deobfuscation.go 已实现的解码器
			return decoded + cmd[i+1:]
		}
	}
	return cmd // 无闭合引号,不解码(保守)
}

// dequoteExecutable 去引号(v1 简化:剥首尾单/双引号对)。
func dequoteExecutable(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if len(cmd) >= 2 {
		if (cmd[0] == '"' && cmd[len(cmd)-1] == '"') || (cmd[0] == '\'' && cmd[len(cmd)-1] == '\'') {
			return cmd[1 : len(cmd)-1]
		}
	}
	return cmd
}

var normPathNormalizerRe = regexp.MustCompile(`^(?:(?:/(?:\S*/)*s?bin/)|(?:[A-Za-z]:[/\\](?:[^\s/\\]*[/\\])*))(rm|git|find|unlink|truncate|shred|tar|dd|mv|chmod|chown|cp|ln|mkdir|mkdisk|diskpart|format|vssadmin|reg|sc)(?i:\.exe|\.com)?(\s|$)`)

// expandPath 展开 /usr/bin/git → git 等。
func expandPath(cmd string) string {
	m := normPathNormalizerRe.FindStringSubmatchIndex(cmd)
	if m == nil {
		return cmd
	}
	// m[2]:m[3] = 捕获组1(命令名),m[4]:m[5] = 捕获组2(空格或结尾)
	name := cmd[m[2]:m[3]]
	tail := cmd[m[4]:m[5]]
	return name + tail + cmd[m[5]:]
}
