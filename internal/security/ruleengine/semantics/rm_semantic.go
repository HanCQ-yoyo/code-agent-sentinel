package semantics

import (
	"regexp"
	"strings"
)

// rmCmdRe 粗匹配 rm 命令起始(含 sudo/env wrapper 前缀)。
// 用 Go raw string(反引号):反斜杠是字面量,所以正则元字符用单反斜杠 \w \s \S \b。
// (brief 原文用 `\\w` + raw string 会匹配字面量反斜杠,是 bug —— 同 Task 8 修正,
// 此处用单反斜杠。)
var rmCmdRe = regexp.MustCompile(`(?:^|[^\w-])(?:sudo\s+(?:\S+\s+)*|env\s+(?:[A-Za-z_]\w*=\S+\s+)+)?rm\b\s*(.*)`)

// RmSemanticDecision 对 rm 命令做 argv 语义解析。
// 对照 dcg core/filesystem.rs:529 parse_rm_command(精简:flag 扫描 + interactive + 管道 stdin)。
//
// 解决正则无法区分的场景:
//   - rm -i file → Safe(interactive,用户逐个确认)
//   - echo y | rm -i file → Deny(管道 stdin 自动确认 -i,绕过 interactive 屏障)
//   - rm -- -rf → NOT Deny(-- 后 -rf 是文件名,非 flag)
//   - rm -r -f / → Deny(flag 拆分,单条 -rf 正则可能漏)
//   - rm -rf / → Deny(递归强制删根)
func RmSemanticDecision(command string) SemanticResult {
	// 用 FindStringSubmatchIndex 拿 rm 起始位置 + 捕获组范围(比 brief 的
	// strings.Index(command," rm ") 更稳健:后者要求 rm 两侧有空格,
	// echo y|rm -i(管道无空格)会漏报 pipe stdin)。
	loc := rmCmdRe.FindStringSubmatchIndex(command)
	if loc == nil {
		return SemanticResult{Decision: Unknown}
	}
	// loc: [整体起始, 整体结束, 组1起始, 组1结束, ...]
	// 组1 = rm 之后的参数(rm\s* 之后的 .*)。loc[2] 是组1起始,即 rm 关键字 + 尾随
	// 空白之后的第一个字符。command[:loc[2]] 包含 rm 及其前缀;
	// 正则 (?:^|[^\w-]) 会吞掉 rm 紧邻前的非 word 字符(如 |),但该字符也在 loc[2] 之前,
	// 故检查 command[:loc[2]] 是否含 | 即可判定 rm 前是否有管道(| 不会出现在 rm\s* 里)。
	hasPipeStdin := strings.Contains(command[:loc[2]], "|")

	rest := command[loc[2]:loc[3]]
	args := strings.Fields(rest)
	if len(args) == 0 {
		return SemanticResult{Decision: Unknown}
	}

	// 扫 flag(遇到 -- 停止 flag 解析,之后是文件名)。
	// 长选项(--recursive / --force / --interactive)与短聚簇(-rf / -fr / -i)分开处理:
	// brief 原实现把 --recursive 当短聚簇逐字符扫描,'r' 误设 recursive、'i' 误设 interactive
	// (致命 false-positive:rm --recursive 被误判为 interactive 安全)。修正:长选项整体比较。
	recursive := false
	force := false
	interactive := false
	dashDashSeen := false
	var targets []string
	for _, a := range args {
		if dashDashSeen {
			targets = append(targets, a)
			continue
		}
		if a == "--" {
			dashDashSeen = true
			continue
		}
		// 长 flag:--recursive / --force / --interactive(整体匹配,不逐字符扫描)。
		// brief 原实现把 --recursive 当短聚簇逐字符扫描,'r' 误设 recursive、'i' 误设 interactive
		// (致命 false-positive:rm --recursive 被误判为 interactive 安全)。修正:长选项整体比较。
		if strings.HasPrefix(a, "--") && len(a) > 2 {
			switch {
			case a == "--recursive":
				recursive = true
			case a == "--force":
				force = true
			case a == "--interactive" || strings.HasPrefix(a, "--interactive="):
				interactive = true
			}
			continue
		}
		// 短 flag 聚簇:-rf / -fr / -i / -r / -f(逐字符扫描)。
		// -R(大写)也表 recursive(GNU rm -R 等价 -r)。
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			for _, f := range a[1:] {
				switch f {
				case 'r', 'R':
					recursive = true
				case 'f':
					force = true
				case 'i':
					interactive = true
				}
			}
			continue
		}
		targets = append(targets, a)
	}

	// interactive + 管道 stdin 自动确认 → 危险(绕过 interactive 屏障)。
	// RuleID 用 filesystem.rm-rf-general:dcg 规则集无专门的 pipe-interactive 规则,
	// 复用通用 rm 破坏性规则(Task 11 接入时语义 Deny 会抑制正则重复报)。
	if interactive && hasPipeStdin {
		return SemanticResult{Decision: Deny, RuleID: "filesystem.rm-rf-general", Reason: "rm -i 管道 stdin 自动确认(绕过 interactive 屏障)"}
	}
	// interactive 无管道 → 安全(用户逐个确认)。
	if interactive {
		return SemanticResult{Decision: Safe, Reason: "rm -i interactive(用户逐个确认)"}
	}
	// 递归+强制 → Deny。按目标是否 root/home 选 RuleID:
	//   - root/home → filesystem.rm-rf-root-home(critical,极端危险)
	//   - 其他路径 → filesystem.rm-rf-general(high,破坏性递归强制)
	// 聚簇(-rf)与拆分(-r -f)在此统一判定,不区分 RuleID(语义层目的就是补正则拆分漏报)。
	if recursive && force {
		for _, t := range targets {
			if isRootOrHome(t) {
				return SemanticResult{Decision: Deny, RuleID: "filesystem.rm-rf-root-home", Reason: "rm -rf " + t + "(递归强制删根/home)"}
			}
		}
		return SemanticResult{Decision: Deny, RuleID: "filesystem.rm-rf-general", Reason: "rm -rf 强制递归(非根目标)"}
	}
	// 非强制非递归的 rm → 交回正则。
	return SemanticResult{Decision: Unknown}
}

// isRootOrHome 判断路径是否为根/家目录(GNU rm 危险目标)。
// / ~ /* /home /home/... 均视为根/home;其他(/tmp/x 等)不是。
func isRootOrHome(p string) bool {
	return p == "/" || p == "~" || strings.HasPrefix(p, "/*") || p == "/home" || strings.HasPrefix(p, "/home/")
}
