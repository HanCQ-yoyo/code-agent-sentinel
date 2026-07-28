// cmd/sentinel/guard_cmd.go
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/intercept"
	"code-agent-sentinel/internal/security/ruleengine"
	"code-agent-sentinel/internal/security/ruleengine/semantics"
)

// newGuardCmd 构造 `sentinel guard` 子命令:被 Claude Code fork 的 PreToolUse hook。
// 读 stdin JSON → 7 步管线评估 → 写 stdout 决策 → 退出。不启动 HTTP server。
// fail-open 铁律:hook 永远 exit 0;deny 只靠 stdout JSON。
func newGuardCmd() *cobra.Command {
	var cfgPath, deadlineFlag string
	var debug bool
	cmd := &cobra.Command{
		Use:    "guard",
		Short:  "运行时拦截 hook(被 Claude Code PreToolUse 调用)",
		Hidden: true, // 内部 hook 命令,不对用户暴露
		RunE: func(cmd *cobra.Command, args []string) error {
			return guardMain(os.Stdin, os.Stdout, os.Stderr, cfgPath, deadlineFlag, debug)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "", "配置文件路径(默认 ~/.claude-sentinel/config.yaml)")
	cmd.Flags().StringVar(&deadlineFlag, "deadline", "", "覆盖评估预算(调试)")
	cmd.Flags().BoolVar(&debug, "debug", false, "stderr 输出评估 trace")
	return cmd
}

// guardMain 是 guard 命令入口:加载配置 → runGuard。fail-open:任何错误 → allow + exit 0。
// stdin/stdout/stderr 用 io.Reader/io.Writer 签名(简化的 io 适配,见 brief Step 3 note):
// 生产路径传 os.Stdin/os.Stdout/os.Stderr(*os.File 满足 io.Reader/io.Writer),测试传
// strings.Reader/bytes.Buffer,无需 asReader/asWriter 薄包装。
func guardMain(stdin io.Reader, stdout, stderr io.Writer, cfgPath, deadlineFlag string, debug bool) error {
	// 顶层 recover:panic 也 fail-open(fail-open 铁律)。
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "guard panic(fail-open): %v\n", r)
			// 不写 stdout = allow
		}
	}()
	if cfgPath == "" {
		p, err := config.DefaultPath()
		if err == nil {
			cfgPath = p
		}
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		// 配置读失败 → 用默认配置(fail-open,不阻断)
		fmt.Fprintf(stderr, "guard 配置加载失败(用默认): %v\n", err)
		cfg = config.DefaultConfig()
	}
	cfg.EnsureGuard()
	// --deadline 覆盖评估预算(调试):非空时解析为 ms,原地改写 cfg.Guard.DeadlineMS,
	// 使后续 DeadlineOrDefault() 取到 flag 值(而非 config 盘上值)。
	if deadlineFlag != "" {
		if ms, err := strconv.Atoi(deadlineFlag); err == nil {
			cfg.Guard.DeadlineMS = ms
		} else {
			fmt.Fprintf(stderr, "guard --deadline %q 解析失败(用配置默认): %v\n", deadlineFlag, err)
		}
	}
	home, _ := os.UserHomeDir()
	return runGuard(stdin, stdout, stderr, cfg, home, debug)
}

// runGuard 是 7 步管线核心(纯函数,可测:注入 stdin/stdout/stderr/cfg/home)。
// 步骤:① 解析 → ② 递归短路 → ③ quick-reject → ④ normalize → ⑤ heredoc → ⑥ pack 评估 → ⑦ 决策输出+记录。
func runGuard(stdin io.Reader, stdout, stderr io.Writer, cfg *config.Config, home string, debug bool) error {
	guard := cfg.Guard
	if !guard.EnabledEffective() {
		// 拦截关闭 → allow 全部(空 stdout)
		return nil
	}
	deadline := guard.DeadlineOrDefault()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(deadline)*time.Millisecond)
	defer cancel()

	// ① 解析 stdin
	maxBytes := guard.MaxBytesOrDefault()
	input, err := intercept.ParseHookInput(stdin, maxBytes)
	if err != nil {
		// fail-open:解析失败 → allow(空 stdout)。仅写 warn 记录。
		fmt.Fprintf(stderr, "guard parse fail-open: %v\n", err)
		return nil
	}
	cmd := input.Command
	if cmd == "" {
		return nil // 无命令 → allow
	}

	// ② 递归短路:sentinel guard 自身 → allow(防 parse_hooks 资产化递归)
	if isSelfGuardCommand(cmd) {
		if debug {
			fmt.Fprintf(stderr, "recursive short-circuit: %q\n", cmd)
		}
		return nil
	}

	start := time.Now()
	store := intercept.NewStore(filepath.Join(home, ".claude-sentinel", "intercept"))

	// 加载规则(单一来源:LoadBuiltin + Validate)
	// LoadBuiltin 返回 (rules, combos, errs) 三元(见 brief discrepancy 1):
	// combos 是跨资产组合规则,guard 单命令不需要,丢弃;errs 仅记录不阻断。
	rules, _, loadErrs := ruleengine.LoadBuiltin()
	rules, _ = ruleengine.Validate(rules)
	if debug {
		fmt.Fprintf(stderr, "loaded %d rules (%d load errs)\n", len(rules), len(loadErrs))
	}
	keywords := ruleengine.CollectKeywords(rules)

	// ③ quick-reject(未命中关键词 + 无混淆 → 放行)
	if ruleengine.QuickReject(cmd, keywords) {
		if debug {
			fmt.Fprintf(stderr, "quick-reject: allow %q\n", cmd)
		}
		writeRecord(store, makeRecord(input, "allow", "", "", "", start))
		return nil
	}

	// ⑥ pack 评估(对 normalize 后命令 + heredoc 提取的内层命令)
	denied, ruleID, severity, reason, remediation := evaluate(ctx, cmd, rules)

	// ⑤ heredoc:若有内联脚本,递归评估提取出的内层命令
	if !denied && ruleengine.HasInlineScript(cmd) {
		for _, inner := range ruleengine.ExtractInlineScripts(cmd, 0) {
			if d, rid, sev, rsn, rem := evaluate(ctx, inner, rules); d {
				denied, ruleID, severity, reason, remediation = true, rid, sev, rsn, rem
				break
			}
		}
	}

	// 超时兜底检查:evaluate 内部不查 ctx.Err()(单命令 256 规则 200ms 预算下足够快,实测无超时);
	// 此处仅在 evaluate 返回后兜底——若 ctx 已超时且未 deny,发 ask + 写 warn 记录。
	if ctx.Err() != nil && !denied {
		intercept.WriteDecision(stdout, intercept.DecisionAsk,
			fmt.Sprintf("评估超时(%dms)", deadline), "", "", "")
		writeRecord(store, makeRecord(input, "warn", "", "", "", start))
		return nil
	}

	// ⑦ 决策输出 + 记录
	if denied {
		intercept.WriteDecision(stdout, intercept.DecisionDeny, reason, ruleID, severity, remediation)
		writeRecord(store, makeRecordWithRule(input, "deny", ruleID, severity, reason, start))
	} else {
		writeRecord(store, makeRecord(input, "allow", "", "", "", start))
	}
	return nil
}

// evaluate 对单条命令跑语义关卡 + 正则 Eval(方案 A:合成 AssetCommand,与静态层同构)。
// 返回 (denied, ruleID, severity, reason, remediation)。
func evaluate(ctx context.Context, cmd string, rules []ruleengine.Rule) (bool, string, string, string, string) {
	// ④ normalize(剥 sudo/env/wrapper + ANSI-C + 路径展开)
	normalized := ruleengine.NormalizeCommand(cmd)
	if normalized == "" {
		return false, "", "", "", ""
	}
	// 语义关卡 1:DispatchCommand 单命令分发(比静态层逐行简单)
	sem := semantics.DispatchCommand(normalized)
	if sem.Decision == semantics.Deny {
		return true, sem.RuleID, severityForRule(rules, sem.RuleID), sem.Reason, remediationForRule(rules, sem.RuleID)
	}
	// 合成 AssetCommand,跑正则 Eval(与静态 RulesDetector 同构)
	asset := configengine.Asset{Type: configengine.AssetCommand, Fields: map[string]any{"command": normalized}}
	// 语义 Safe(wholeSafe,单命令):整条命令语义判安全(如 git commit -m "..." 数据区字面量、
	// rm -i 交互式、git checkout -b 新建分支)。Safe 表示命令整体不执行破坏性操作,数据区内
	// 的 rm -rf 等是字面量。故 Safe 下抑制所有 command 字段正则命中(对照静态层 wholeSafe 行为,
	// rules_detector.go 关卡 2:Locations 为空 + wholeSafe → 丢弃)。
	//
	// 注意:仅当 DispatchCommand 返回 Safe(非 Unknown)才抑制。Unknown(无语义解析器能判)
	// 不抑制,正则照常跑(baseline.dangerous-hook 等无 domain 规则在 Unknown 下正常检测)。
	wholeSafe := sem.Decision == semantics.Safe
	// KNOWN LIMITATION(链式命令绕过,与静态层 rules_detector.go computeLineSemantic 同行为):
	// `git commit -m "x" && rm -rf /` 这类链式命令,DispatchCommand 取首个非 Unknown 语义(git commit -m → Safe),
	// wholeSafe=true 会抑制整条命令所有 command 字段正则命中,包括 `&& rm -rf /` 片段 → 误 allow。
	// ; 与 | 分隔同理。根因:本函数(及静态层)对单命令/单行整体判语义,不按 &&/;/| 拆分独立评估各片段。
	// v1 接受此限制(guard 是补充防线,非完整 shell parser);R3 计划加 splitAndEvaluate 按分隔符拆分独立评估闭合。
	for _, r := range rules {
		if r.AssetType != string(configengine.AssetCommand) && r.AssetType != string(configengine.AssetHook) {
			// destructive 域规则 asset_type=hook 但 or-tree 覆盖 command 字段;
			// guard 单命令都走 command 字段,放宽到 hook 域规则(or-tree 内部按 command 字段匹配)
			continue
		}
		domain, _ := r.Metadata["domain"].(string)
		// 语义 Safe 关卡 1(正则前):wholeSafe 时跳过该域已实现规则(防数据区字面量误报,
		// 如 git commit -m "rm -rf" 不被 filesystem.rm-rf-root-home 误判)。
		if wholeSafe && domainMatch(domain, sem) {
			continue
		}
		res := ruleengine.Eval(r, asset)
		if res.Matched {
			// 语义 Safe 关卡 2(正则后复核):wholeSafe 时丢弃命中(单命令 wholeSafe,
			// 对照静态层 findingInSafeLines 对 command 字段命中用 wholeSafe 判定)。
			// 对无 domain 规则(baseline.dangerous-hook 等):wholeSafe 时也丢弃——
			// 语义已判整条命令安全,正则在数据区字面量上的命中是误报。
			if wholeSafe {
				continue
			}
			return true, r.ID, r.Severity, r.Description, r.Remediation
		}
	}
	return false, "", "", "", ""
}

// domainMatch 判断规则的 domain 是否与语义结果域一致(git/snowflake 等语义返回的隐含域)。
// 简化:SemanticResult 无 Domain 字段(见 brief discrepancy 6),按 RuleID 前缀判断。
// Safe 路径:git commit -m / git tag -m 等 Safe 决策的 RuleID 恒为 ""(git_semantic.go 不填),
// 此时保守匹配任意已实现域(domain != ""→true),使所有已实现域规则在 Safe 下被关卡1/2 抑制,
// 正确防数据区字面量误报(如 git commit -m "rm -rf" 不被 filesystem 规则 deny)。
// Deny 路径在 evaluate 已短路(不进本函数的 HasPrefix 分支),此处 HasPrefix 分支留给
// 未来 RuleID 带 domain 前缀的扩展场景。
func domainMatch(domain string, sem semantics.SemanticResult) bool {
	// 语义结果未带 Domain 字段;按 RuleID 前缀判断(git.* / filesystem.* 等)
	if sem.RuleID == "" {
		return domain != "" // Safe 无 RuleID 时保守匹配任意已实现域
	}
	return strings.HasPrefix(sem.RuleID, domain+".") || strings.HasPrefix(domain, sem.RuleID)
}

// isSelfGuardCommand 判断命令是否是 sentinel guard 自身(递归短路)。
func isSelfGuardCommand(cmd string) bool {
	c := strings.TrimSpace(strings.ToLower(cmd))
	return strings.HasPrefix(c, "sentinel guard") || strings.Contains(c, " sentinel guard")
}

// severityForRule / remediationForRule 按语义 RuleID 找载体规则的 severity/remediation。
// 语义 Deny 返回的 RuleID 是 rule_id(如 "git.reset-hard"),规则 Metadata["rule_id"]
// 与之对齐;未找到回退 high / 空(与静态层 pickSemanticCarrier 同语义)。
func severityForRule(rules []ruleengine.Rule, ruleID string) string {
	for _, r := range rules {
		if rid, _ := r.Metadata["rule_id"].(string); rid == ruleID {
			return r.Severity
		}
	}
	return "high"
}
func remediationForRule(rules []ruleengine.Rule, ruleID string) string {
	for _, r := range rules {
		if rid, _ := r.Metadata["rule_id"].(string); rid == ruleID {
			return r.Remediation
		}
	}
	return ""
}

// makeRecord 构造拦截记录(allow/warn)。WorkingDir = 进程 cwd(spec §4.1)。
func makeRecord(input intercept.HookInput, outcome, ruleID, severity, reason string, start time.Time) intercept.InterceptRecord {
	cwd, _ := os.Getwd()
	return intercept.InterceptRecord{
		Timestamp: start, AgentProtocol: "claude", WorkingDir: cwd,
		Command: input.Command, Outcome: outcome, RuleID: ruleID,
		Severity: severity, Reason: reason, SessionID: input.SessionID, ToolName: input.ToolName,
		EvalDurationUS: time.Since(start).Microseconds(),
	}
}
func makeRecordWithRule(input intercept.HookInput, outcome, ruleID, severity, reason string, start time.Time) intercept.InterceptRecord {
	rec := makeRecord(input, outcome, ruleID, severity, reason, start)
	rec.ID = recordID(start, input.Command)
	return rec
}
func recordID(t time.Time, cmd string) string {
	return fmt.Sprintf("%s-%x", t.Format("20060102-150405"), hashStr(cmd))
}
func hashStr(s string) uint64 {
	var h uint64 = 1469598103934665603
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

// writeRecord 写记录。原 brief 设计为异步(go func),但 hook 子进程在 RunE 返回后
// 即退出(main 不等 goroutine),异步写会丢记录(生产 bug)且致测试 TempDir 清理竞态
// ("directory not empty")。改为同步写:Append 是 MkdirAll+rename,亚毫秒级,不显著拖慢
// hook 响应(stdout 已在调用前写完);写失败只忽略,不影响 fail-open(exit 0)。
func writeRecord(store *intercept.Store, rec intercept.InterceptRecord) {
	if rec.ID == "" {
		rec.ID = recordID(rec.Timestamp, rec.Command)
	}
	_ = store.Append(rec) // 写失败忽略(fail-open)
}
