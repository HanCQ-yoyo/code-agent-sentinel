package semantics

import (
	"regexp"
	"strings"
)

// Decision 是语义解析结果。
type Decision int

const (
	Unknown Decision = iota // 无法判定,交回正则层
	Safe                    // 安全子命令,跳过该命令(数据区/安全操作)
	Deny                    // 破坏性,直接报
)

// SemanticResult 是语义解析返回。
type SemanticResult struct {
	Decision Decision
	RuleID   string // 对齐 dcg rule_id(如 git.branch-force-delete);Deny 时填
	Reason   string
}

// gitCmdRe 粗匹配 git 命令起始(含 sudo/env wrapper 前缀)。
// 用 Go raw string(反引号):反斜杠是字面量,所以正则元字符用单反斜杠 \w \s \S。
// (brief 原文用 `\\w` + raw string 会匹配字面量反斜杠,是 bug —— 此处修正为单反斜杠。)
var gitCmdRe = regexp.MustCompile(`(?:^|[^\w-])(?:sudo\s+(?:\S+\s+)*|env\s+(?:[A-Za-z_]\w*=\S+\s+)+)?git\s+(.*)`)

// GitSemanticDecision 对 git 命令做语义判断。
// 精简版(对照 dcg core/git.rs 5668 行裁剪到核心):
//  1. 提取 git 子命令 + args
//  2. 破坏子命令(branch -D / reset --hard / checkout -- / clean -f / push -f / stash drop)→ Deny
//  3. commit -m / tag -m / merge -m → Safe(数据区命令字面量不报)
//  4. checkout -b / --orphan / restore --staged → Safe
//  5. 其余 → Unknown(走正则)
//
// alias 展开留 v2:dcg 递归展开 git alias(git cleanup→branch -D),需读 .git/config,
// 在 configengine 无副作用约束下不宜实现。若 alias 漏报是刚需,Task 11 接入时补。
func GitSemanticDecision(command string) SemanticResult {
	m := gitCmdRe.FindStringSubmatch(command)
	if m == nil {
		return SemanticResult{Decision: Unknown}
	}
	rest := strings.TrimSpace(m[1])
	// 去掉 git 全局 flag(--git-dir 等直到子命令)
	rest = stripGitGlobalFlags(rest)
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return SemanticResult{Decision: Unknown}
	}
	sub := parts[0]
	args := parts[1:]

	switch sub {
	case "commit", "tag", "merge":
		// -m 后是数据区,内含的 rm -rf 等是字面量不执行 → Safe
		if hasFlag(args, "-m", "--message") {
			return SemanticResult{Decision: Safe, Reason: sub + " -m 数据区命令字面量不执行"}
		}
		return SemanticResult{Decision: Unknown}
	case "checkout":
		if hasFlag(args, "-b", "-B", "--orphan") {
			return SemanticResult{Decision: Safe, Reason: "checkout 新建分支安全"}
		}
		if hasDashDash(args) { // checkout -- <path> 丢弃
			return SemanticResult{Decision: Deny, RuleID: "git.checkout-discard", Reason: "checkout -- 丢弃改动"}
		}
		return SemanticResult{Decision: Unknown}
	case "reset":
		if hasFlag(args, "--hard", "--merge") {
			return SemanticResult{Decision: Deny, RuleID: "git.reset-hard", Reason: "reset --hard/merge 丢弃改动"}
		}
		return SemanticResult{Decision: Unknown}
	case "branch":
		if hasFlag(args, "-D", "--delete", "-d") {
			return SemanticResult{Decision: Deny, RuleID: "git.branch-force-delete", Reason: "branch 删除"}
		}
		return SemanticResult{Decision: Unknown}
	case "clean":
		if hasFlag(args, "-f", "--force") {
			return SemanticResult{Decision: Deny, RuleID: "git.clean-force", Reason: "clean -f 强制清理"}
		}
		return SemanticResult{Decision: Unknown}
	case "push":
		if hasFlag(args, "-f", "--force", "--force-with-lease") {
			return SemanticResult{Decision: Deny, RuleID: "git.push-force-short", Reason: "push -f 强推"}
		}
		return SemanticResult{Decision: Unknown}
	case "stash":
		if len(args) > 0 && (args[0] == "drop" || args[0] == "clear") {
			return SemanticResult{Decision: Deny, RuleID: "git.stash-drop", Reason: "stash drop/clear"}
		}
		return SemanticResult{Decision: Unknown}
	case "restore":
		if hasFlag(args, "--staged", "-S") && !hasFlag(args, "--worktree", "-W") {
			return SemanticResult{Decision: Safe, Reason: "restore --staged 仅影响索引"}
		}
		return SemanticResult{Decision: Unknown}
	}
	return SemanticResult{Decision: Unknown}
}

// stripGitGlobalFlags 去掉 -C <path> / --git-dir 等全局 flag(它们在子命令前)。
func stripGitGlobalFlags(s string) string {
	parts := strings.Fields(s)
	var out []string
	skipNext := false
	for _, p := range parts {
		if skipNext {
			skipNext = false
			continue
		}
		if p == "-C" || p == "--git-dir" || p == "--work-tree" {
			skipNext = true
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, " ")
}

func hasFlag(args []string, flags ...string) bool {
	for _, a := range args {
		for _, f := range flags {
			// 精确匹配(含 --long-flag=value 形式)。
			if a == f || strings.HasPrefix(a, f+"=") {
				return true
			}
			// POSIX 短 flag 聚簇:单 char flag(如 -f)出现在 -fd / -df 等聚簇里。
			// 仅对单 dash + 单字符 flag(形如 "-f")做聚簇展开,长 flag(--force)不展开。
			if isShortFlag(f) && isShortCluster(a) {
				if strings.Contains(a[1:], f[1:]) {
					return true
				}
			}
		}
	}
	return false
}

// isShortFlag 判断 f 是否为单 dash + 单字符的短 flag(如 "-f","-d")。
func isShortFlag(f string) bool {
	return len(f) == 2 && f[0] == '-' && f[1] != '-'
}

// isShortCluster 判断 a 是否为短 flag 聚簇(单 dash 开头,后跟多字符,如 "-fd")。
// "--force" 等长 flag 不算聚簇。
func isShortCluster(a string) bool {
	return len(a) >= 3 && a[0] == '-' && a[1] != '-'
}

func hasDashDash(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return true
		}
	}
	return false
}
