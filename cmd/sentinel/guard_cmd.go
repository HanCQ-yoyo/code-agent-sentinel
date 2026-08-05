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
	"code-agent-sentinel/internal/storage"
)

// newGuardCmd 构造 `code-agent-sentinel guard` 子命令:被 Claude Code fork 的 PreToolUse hook。
// 读 stdin JSON → 7 步管线评估 → 写 stdout 决策 → 退出。不启动 HTTP server。
// fail-open 铁律:hook 永远 exit 0;deny 只靠 stdout JSON。
func newGuardCmd() *cobra.Command {
	var cfgPath, deadlineFlag string
	var debug bool
	cmd := &cobra.Command{
		Use:    "guard",
		Short:  "Runtime intercept hook (called by Claude Code PreToolUse)",
		Hidden: true, // 内部 hook 命令,不对用户暴露
		RunE: func(cmd *cobra.Command, args []string) error {
			return guardMain(os.Stdin, os.Stdout, os.Stderr, cfgPath, deadlineFlag, debug)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "", "Config file path (default ~/.code-agent-sentinel/config.yaml)")
	cmd.Flags().StringVar(&deadlineFlag, "deadline", "", "Override evaluation budget (debug)")
	cmd.Flags().BoolVar(&debug, "debug", false, "Output evaluation trace to stderr")
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
		fmt.Fprintf(stderr, "guard config load failed (using defaults): %v\n", err)
		cfg = config.DefaultConfig()
	}
	cfg.EnsureGuard()
	// --deadline 覆盖评估预算(调试):非空时解析为 ms,原地改写 cfg.Guard.DeadlineMS,
	// 使后续 DeadlineOrDefault() 取到 flag 值(而非 config 盘上值)。
	if deadlineFlag != "" {
		if ms, err := strconv.Atoi(deadlineFlag); err == nil {
			cfg.Guard.DeadlineMS = ms
		} else {
			fmt.Fprintf(stderr, "guard --deadline %q parse failed (using config default): %v\n", deadlineFlag, err)
		}
	}
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".code-agent-sentinel", "sentinel.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		db = nil // fail-open: DB 不可用时用 builtin 规则,allowlist 为空
	}
	if db != nil {
		defer db.Close()
	}
	return runGuard(stdin, stdout, stderr, cfg, home, db, debug)
}

// runGuard 是 7 步管线核心(纯函数,可测:注入 stdin/stdout/stderr/cfg/home/db/debug)。
// 步骤:① 解析 → ② 递归短路 → ③ quick-reject → ④ normalize → ⑤ 链式拆分 + 片段评估
//
//	→ ⑤' heredoc 兜底 → ⑦ allowlist 双匹配 → ⑧ 决策聚合 → ⑨ 超时兜底 → ⑩ 输出+记录。
//
// R3 重构:evaluate 从「单命令 wholeSafe」改为「片段级 span+语义+正则+confidence」(I1 闭合)。
// Task 9:规则来源从 LoadBuiltin 改为优先读 sqlite 拦截规则(db 指向 sentinel.db);
// db 打不开/读失败/空 → fail-open 回退 LoadBuiltin(铁律:存储故障不致检测失效、不误 deny)。
// R3 evaluateSegment pipeline(span+confidence+Mode+allowlist)不变,只改规则加载来源。
func runGuard(stdin io.Reader, stdout, stderr io.Writer, cfg *config.Config, home string, db *storage.DB, debug bool) error {
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
		// fail-open:解析失败 → allow(空 stdout)。仅写 stderr 日志(不写记录:无合法 input)。
		fmt.Fprintf(stderr, "guard parse fail-open: %v\n", err)
		return nil
	}
	cmd := input.Command
	if cmd == "" {
		return nil // 无命令 → allow
	}

	// ② 递归短路:code-agent-sentinel guard 自身 → allow(防 parse_hooks 资产化递归)
	if isSelfGuardCommand(cmd) {
		if debug {
			fmt.Fprintf(stderr, "recursive short-circuit: %q\n", cmd)
		}
		return nil
	}

	start := time.Now()
	store := intercept.NewStore(db)

	// 协议探测(① 的延伸):按 stdin 的 tool_name + turn_id 消歧 Claude/Codex。
	// 评估管线对两协议完全相同,仅 ⑩ 输出形态不同(见 WriteDecision 的 proto 参数)。
	proto := intercept.DetectProtocol(input.ToolName, input.TurnID)

	// 加载规则(Task 9):优先读 sqlite 拦截规则(用户自定义拦截规则第一次能生效);
	// dbPath 空/打不开/读失败/读空 → fail-open 回退 LoadBuiltin(铁律:存储故障不致检测失效、不误 deny)。
	//
	// fail-open 设计:LoadInterceptRules 返回 (rules, errs)——errs 是单条规则的反序列化/编译错误,
	// 非「读 db 失败」;读 db 失败(如 corrupt、表不存在)时 ListRulesEnabled 返回 error,
	// LoadInterceptRules 把它包进 errs 并返回 nil rules → len(rules)==0 触发回退。
	// 故回退判定用 len(rules)==0(而非检查某个 err 变量),覆盖所有「db 不可用」子情况:
	//   - dbPath 空(旧测试/无 db 环境)→ 跳过 Open,直接回退(行为等价改造前)
	//   - Open 失败(文件不存在/损坏 "file is not a database")→ 跳过 Load,回退
	//   - Open 成功但 ListRulesEnabled 失败(表未迁移/损坏)→ Load 返回 nil,回退
	//   - Open 成功但表空(Task 10 sync 尚未跑)→ Load 返回空 rules,回退
	// 注意:storage.Open 对「非 sqlite 文件」会 Ping 失败并返回 (nil, err)(已实测),不会 panic;
	// 即便某路径异常 panic,guardMain 顶层 recover 兜底 fail-open allow(铁律双保险)。
	var rules []ruleengine.Rule
	var loadErrs []ruleengine.RuleLoadError
	if db != nil {
		rules, loadErrs = ruleengine.LoadInterceptRules(db)
	}
	if len(rules) == 0 {
		// fail-open 回退 builtin(db 无/打不开/读空 → 用 embed 规则,不致检测失效)。
		// 只加载 destructive_commands.yaml:拦截只该用破坏性命令规则,与 db 路径
		// (LoadInterceptRules 只读 intercept 域,而 intercept 域只同步了 destructive)对齐。
		rules, _, loadErrs = ruleengine.LoadInterceptBuiltin()
		if debug {
			fmt.Fprintf(stderr, "guard fallback to builtin (db unavailable)\n")
		}
	}
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
		writeRecord(store, makeRecord(input, proto, "allow", "", "", "", start))
		return nil
	}

	mode := guard.ModeOrDefault()

	// ⑤ 链式拆分(I1):按 &&/;/||/| 拆片段,每片段独立评估
	var results []segmentResult
	for _, seg := range ruleengine.SplitCommand(cmd) {
		results = append(results, evaluateSegment(ctx, seg, rules, mode))
	}

	// ⑤' heredoc 兜底:若 ⑥ 未 deny 且有内联脚本,对提取的内层命令回灌评估
	// (每条内层再 SplitCommand,闭合 `bash -c "safe && rm -rf /"` 这类 heredoc 内链式)
	anyDenied := false
	for _, r := range results {
		if r.denied {
			anyDenied = true
			break
		}
	}
	if !anyDenied && ruleengine.HasInlineScript(cmd) {
		for _, inner := range ruleengine.ExtractInlineScripts(cmd, 0) {
			for _, seg := range ruleengine.SplitCommand(inner) {
				r := evaluateSegment(ctx, seg, rules, mode)
				if r.denied {
					results = append(results, r)
					anyDenied = true
					break
				}
			}
			if anyDenied {
				break
			}
		}
	}

	// ⑦ allowlist 双匹配(原始命令 + normalize 后命令,整条命令各 Matches 一次)
	// 任一命中 → allow + 写 allow 记录。安全不变量:只精确整条匹配,不做通配/正则。
	allowlist := config.NewAllowlistStore(db)
	if guard.AllowlistEnabledOrDefault() && allowlist != nil {
		normCmd := ruleengine.NormalizeCommand(cmd)
		if allowlist.Matches(cmd) || allowlist.Matches(normCmd) {
			writeRecord(store, makeRecord(input, proto, "allow", "", "", "", start))
			return nil // allowlist 命中 → allow
		}
	}

	// ⑧ 决策聚合:High deny → deny;Low deny → ask;无 deny → allow
	decision, ruleID, severity, reason, remediation, confidence, matchedSpan := aggregate(results, mode)

	// ⑨ 超时兜底:evaluateSegment 内部不查 ctx.Err();此处兜底——若 ctx 已超时且未 deny,
	// 发 ask + 写 warn 记录(Codex 下 ask 在 WriteDecision 内退化为 deny,见 protocol.go)。
	if ctx.Err() != nil && decision != intercept.DecisionDeny {
		intercept.WriteDecision(stdout, proto, intercept.DecisionAsk,
			fmt.Sprintf("evaluation timeout (%dms)", deadline), "", "", "")
		writeRecord(store, makeRecordWithConfidence(input, proto, "warn", "", "", "", "", "", start))
		return nil
	}

	// ⑩ 输出(按协议分支)+ 记录
	if decision == intercept.DecisionDeny {
		intercept.WriteDecision(stdout, proto, intercept.DecisionDeny, reason, ruleID, severity, remediation)
		writeRecord(store, makeRecordWithConfidence(input, proto, "deny", ruleID, severity, reason, confidence, matchedSpan, start))
	} else if decision == intercept.DecisionAsk {
		// Codex 下 WriteDecision 内部把 ask 退化为 deny(protocol.go);Claude 发 ask。
		// 记录 outcome=warn:保持 ask 的「低置信度,交用户确认」语义(前端按 outcome 区分展示)。
		intercept.WriteDecision(stdout, proto, intercept.DecisionAsk, reason, ruleID, severity, remediation)
		writeRecord(store, makeRecordWithConfidence(input, proto, "warn", ruleID, severity, reason, confidence, matchedSpan, start))
	} else {
		writeRecord(store, makeRecord(input, proto, "allow", "", "", "", start))
	}
	return nil
}

// segmentResult 是单个片段的评估结果(R3 重构)。
type segmentResult struct {
	denied      bool
	confidence  ruleengine.Confidence
	ruleID      string
	severity    string
	reason      string
	remediation string
	matchedSpan string // 命中片段文本(链式拆分后定位用)
}

// evaluateSegment 对单个片段跑 ⑥a-⑥d(span 分类 + 语义关卡 + 正则 Eval + confidence)。
// 返回 segmentResult;denied=false 表示该片段无破坏性命中。
//
// panic 兜底:本函数 defer recover——ClassifySpans/SplitCommand/ScoreConfidence 自带 panic
// 兜底(各自返回安全默认值,见 span.go/split.go/confidence.go),但 Eval 不自带 recover
// (eval.go 迭代编译好的正则,理论上不易 panic,但无兜底)。为闭合不变量#4(「异常宁可误拦
// 不漏报」),本函数在入口 defer recover:panic 时返回 denied=true + ConfUnknown 的保守
// 拦截结果,交 aggregate() 按 Mode 解释(strict→High→deny,lenient→Low→ask),避免
// panic 冒泡到 guardMain 顶层 recover 导致 fail-open allow(违反不变量#4)。
//
// Location/Span 坐标系对齐(关键设计决策):
//   - ruleengine.Location 是 {Line, StartCol, EndCol} 行列模型(schema.go:110);
//     ruleengine.Span 是 {Start, End} 字节偏移模型(span.go:15)。两者坐标系不同,不能直接比较。
//   - command 字段的 Eval 不产 Location(eval.go:7:仅 content 字段叶子产 Location),
//     故 EvalResult.Locations 对 command 资产恒为空。
//   - 解决方案(spec §3.3 intent + 方案 #3):用 res.Evidence(命中文本)在 normalized
//     命令中 strings.Index 重新定位字节偏移,再查 spans 判定命中落在 Executed/Data/Comment 区。
//     Data/Comment 区命中 → 丢弃(引号内字面量,如 echo "rm -rf /" 的 rm -rf /);
//     Executed 区命中 → 交 ScoreConfidence 打分(中心 High / 紧贴边界 Low)。
//     这绕开了 Location 行列与 Span 字节偏移的坐标系冲突。
func evaluateSegment(ctx context.Context, seg string, rules []ruleengine.Rule, mode string) (result segmentResult) {
	// panic 兜底(不变量#4 闭合):Eval 不自带 recover,若其内部(或本函数任意路径)panic,
	// 不让 panic 冒泡到 guardMain 顶层 recover(fail-open allow),而是返回保守拦截结果:
	// denied=true + ConfUnknown → aggregate() 按 Mode 解释(strict→High→deny / lenient→Low→ask)。
	// 用命名返回值 result,defer 可真正改写返回值(Go defer recover 无法修改非命名返回值)。
	defer func() {
		if r := recover(); r != nil {
			result = segmentResult{
				denied:      true,
				confidence:  ruleengine.ConfUnknown, // 交 aggregate/ForMode 按 Mode 解释
				matchedSpan: seg,                    // seg 是入参,闭包可捕获
				reason:      "evaluation panic, conservative block",
				// ruleID/severity/remediation 留空:panic 时无法确定命中哪条规则
			}
		}
	}()
	if seg == "" {
		return segmentResult{}
	}
	// ④ normalize(剥 sudo/env/wrapper + ANSI-C + 路径展开)
	normalized := ruleengine.NormalizeCommand(seg)
	if normalized == "" {
		return segmentResult{}
	}
	// ⑥a span 分类(基于 normalized 命令,字节偏移坐标系,供 confidence 定位 + Data 区误报剔除)
	// 注意:必须在 normalize 之后分类——normalize 会改写字节内容(如 $'\x72\x6d' → rm),
	// span 的字节偏移与 normalized 的 evidence/offset 才能对齐。若对原始 seg 分类,
	// ANSI-C/反混淆场景的 evidence(在 normalized 中)无法映射回原始 seg 的 span 偏移。
	spans := ruleengine.ClassifySpans(normalized)
	// ⑥b 语义关卡 1:DispatchCommand 单命令分发
	//   - Deny → 直接命中(High,跳过正则,防漏报)
	//   - Safe → 整条命令语义安全(如 git commit -m 数据区字面量、rm -i 交互式),正则命中全部丢弃
	//   - Unknown → 交回正则层
	sem := semantics.DispatchCommand(normalized)
	if sem.Decision == semantics.Deny {
		// span 复核(语义 Deny 也需):语义解析器的正则(gitCmdRe/rmCmdRe)不识别引号边界,
		// 会把 echo "rm -rf /" 引号内的 rm 误判 Deny。用「危险关键字是否在 Executed span 文本中」复核:
		// 拼接所有 SpanExecuted 文本,若关键字不在其中 → 命中纯数据区字面量 → 丢弃(交回正则层/默认 allow)。
		// 关键字按 RuleID 域提取(git.reset-hard → "reset --hard";filesystem.* → "rm")。
		if semanticDenyInDataSpan(normalized, sem, spans) {
			// 数据区字面量:降级为 Unknown 走正则层(正则层会再做 span 复核,Data span 命中丢弃 → allow)
			sem = semantics.SemanticResult{Decision: semantics.Unknown}
		} else {
			return segmentResult{
				denied:      true,
				confidence:  ruleengine.ConfHigh, // 语义 Deny 恒 High(明确命中,不变量#1 两模式都 deny)
				ruleID:      sem.RuleID,
				severity:    severityForRule(rules, sem.RuleID),
				reason:      sem.Reason,
				remediation: remediationForRule(rules, sem.RuleID),
				matchedSpan: seg,
			}
		}
	}
	wholeSafe := sem.Decision == semantics.Safe
	asset := configengine.Asset{Type: configengine.AssetCommand, Fields: map[string]any{"command": normalized}}
	// ⑥c 正则 Eval + span 复核(关卡 2,R3 新:Data/Comment span 命中丢弃)
	for _, r := range rules {
		if r.AssetType != string(configengine.AssetCommand) && r.AssetType != string(configengine.AssetHook) {
			// destructive 域规则 asset_type=hook 但 or-tree 覆盖 command 字段;
			// guard 单命令都走 command 字段,放宽到 hook 域规则
			continue
		}
		domain, _ := r.Metadata["domain"].(string)
		// 语义 Safe 关卡 1(正则前):wholeSafe 时跳过该域已实现规则(防数据区字面量误报)
		if wholeSafe && domainMatch(domain, sem) {
			continue
		}
		res := ruleengine.Eval(r, asset)
		if !res.Matched {
			continue
		}
		// span 复核(关卡 2):命中落在 Data/Comment span → 丢弃(引号内字面量,如 echo "rm -rf /")
		// 用 res.Evidence 在 normalized 中重新定位字节偏移(command 字段 Eval 无 Location)。
		if hitInDataSpan(res.Evidence, normalized, spans) {
			continue
		}
		// 语义 Safe 复核:wholeSafe 且命中未落 Data span(如 git commit -m "x" 的 git commit 部分)
		// → 丢弃(整条命令语义安全)
		if wholeSafe {
			continue
		}
		// ⑥d confidence 打分:命中在 Executed span 内,按距边界距离判 High/Low
		offset := matchOffset(res.Evidence, normalized)
		conf := ruleengine.ScoreConfidence(res.Evidence, offset, spans)
		// ScoreConfidence panic 时返回 ConfUnknown(命名返回值 + deferred recover,见 confidence.go);
		// 不存在 PanicConfidence() 访问器(Task 3 DROPPED),直接读返回的 Confidence。
		// ConfUnknown 按 Mode 解释:strict→High(deny,误拦)/lenient→Low(ask)。
		if conf == ruleengine.ConfUnknown {
			conf = conf.ForMode(mode)
		}
		return segmentResult{
			denied:      true,
			confidence:  conf,
			ruleID:      r.ID,
			severity:    r.Severity,
			reason:      r.Description,
			remediation: r.Remediation,
			matchedSpan: seg,
		}
	}
	return segmentResult{}
}

// hitInDataSpan 判断正则命中是否落在 Data/Comment span(丢弃误报)。
// 用 evidence(命中文本)在 normalized 命令中 strings.Index 重新定位字节偏移——
// command 字段 Eval 不产 Location(eval.go:7),EvalResult.Locations 恒空,无法直接取偏移。
// 这绕开了 Location(行列)与 Span(字节偏移)的坐标系冲突。
func hitInDataSpan(evidence, normalized string, spans []ruleengine.Span) bool {
	if len(spans) == 0 || evidence == "" {
		return false
	}
	offset := matchOffset(evidence, normalized)
	if offset < 0 {
		return false
	}
	matchEnd := offset + len(evidence)
	for _, s := range spans {
		// 命中起点落在 span 内,且命中不跨越 span 边界
		if offset >= s.Start && offset < s.End {
			if s.Kind != ruleengine.SpanExecuted {
				return true // Data/Comment 区:引号内字面量 / 注释
			}
			// Executed 区但命中跨出 span(如跨引号)→ 视为 Data(保守丢弃)
			if matchEnd > s.End {
				return true
			}
			return false
		}
	}
	return false
}

// matchOffset 返回 evidence 在 normalized 命令中的首个字节偏移(无则 -1)。
// command 字段 EvalResult.Locations 恒空(eval.go:7),用 Evidence 重新定位。
func matchOffset(evidence, normalized string) int {
	if evidence == "" || normalized == "" {
		return -1
	}
	return strings.Index(normalized, evidence)
}

// semanticDenyInDataSpan 判断语义 Deny 的危险关键字是否落在 Data/Comment span(丢弃误报)。
// 语义解析器(gitCmdRe/rmCmdRe)的正则不识别引号边界,会把 echo "rm -rf /" 引号内的 rm 误判 Deny。
// 复核策略(方案 #3 的语义层延伸):按 RuleID 域提取危险关键字,检查它是否出现在任意
// SpanExecuted 文本中。若关键字仅出现在 Data/Comment span(引号内字面量)→ true(丢弃)。
//
// 关键字提取(按已实现的语义解析器):
//   - git.* → 子命令关键字(reset/branch/clean/push/stash/checkout 等,从 RuleID 取)
//   - filesystem.* → "rm"(rm 语义解析器只对 rm 关键字判 Deny)
//   - 其余 → 保守 false(未知域不剔除,走 Deny,安全不变量:宁可误拦)
func semanticDenyInDataSpan(normalized string, sem semantics.SemanticResult, spans []ruleengine.Span) bool {
	if len(spans) == 0 || sem.RuleID == "" {
		return false
	}
	keyword := semanticDenyKeyword(sem.RuleID)
	if keyword == "" {
		return false
	}
	// 关键字必须出现在至少一个 SpanExecuted 文本中,否则视为纯数据区字面量
	for _, s := range spans {
		if s.Kind == ruleengine.SpanExecuted && strings.Contains(s.Text, keyword) {
			return false // 关键字在 executed 区,是真命中
		}
	}
	// 关键字不在任何 executed span → 纯 Data/Comment 区字面量 → 丢弃
	return true
}

// semanticDenyKeyword 按 RuleID 返回用于 span 复核的危险关键字(小写)。
// git.reset-hard → "reset";git.branch-force-delete → "branch";filesystem.* → "rm"。
// RuleID 用连字符分隔子修饰(如 git.push-force-short),关键字取 git. 后到首个 - 的部分。
func semanticDenyKeyword(ruleID string) string {
	if strings.HasPrefix(ruleID, "git.") {
		// git.<sub>-... 取点后到首个 - 的部分(git.reset-hard → reset;git.push-force-short → push)
		rest := strings.TrimPrefix(ruleID, "git.")
		if i := strings.Index(rest, "-"); i > 0 {
			return rest[:i]
		}
		return rest
	}
	if strings.HasPrefix(ruleID, "filesystem.") {
		return "rm" // rm 语义解析器只对 rm 关键字判 Deny
	}
	if strings.HasPrefix(ruleID, "snowflake.") {
		return "snow sql" // snowflake 语义解析器只对 snow sql 判 Deny
	}
	return ""
}

// aggregate 聚合片段结果:High deny → deny;Low deny → ask;无 deny → allow。
// 安全不变量:High 命中两模式都 deny(Mode 只影响不确定降级);Low 命中两模式都 ask(交用户确认)。
func aggregate(results []segmentResult, mode string) (decision intercept.Decision, ruleID, severity, reason, remediation, confidence, matchedSpan string) {
	var highDeny, lowDeny *segmentResult
	for i := range results {
		r := &results[i]
		if !r.denied {
			continue
		}
		if r.confidence == ruleengine.ConfHigh {
			highDeny = r
			break // High 优先,直接定 deny
		}
		if lowDeny == nil {
			lowDeny = r // 记首个 Low deny(Low 之间不区分,聚合取首条)
		}
	}
	if highDeny != nil {
		return intercept.DecisionDeny, highDeny.ruleID, highDeny.severity, highDeny.reason, highDeny.remediation, highDeny.confidence.String(), highDeny.matchedSpan
	}
	if lowDeny != nil {
		// Low deny → ask(两模式都 ask,不变量:低置信度交用户确认)
		return intercept.DecisionAsk, lowDeny.ruleID, lowDeny.severity, lowDeny.reason, lowDeny.remediation, lowDeny.confidence.String(), lowDeny.matchedSpan
	}
	return intercept.DecisionAllow, "", "", "", "", "", ""
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

// isSelfGuardCommand 判断命令是否是 code-agent-sentinel guard 自身(递归短路)。
func isSelfGuardCommand(cmd string) bool {
	c := strings.TrimSpace(strings.ToLower(cmd))
	return strings.HasPrefix(c, "code-agent-sentinel guard") || strings.Contains(c, " code-agent-sentinel guard")
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

// makeRecord 构造拦截记录(allow/warn/通用)。WorkingDir = 进程 cwd(spec §4.1)。
// proto 决定 AgentProtocol 字段(claude/codex),从 ① 协议探测传入——R2 写死 "claude",
// R3 改为动态,使 allow 路径的记录也带正确协议。
func makeRecord(input intercept.HookInput, proto intercept.AgentProtocol, outcome, ruleID, severity, reason string, start time.Time) intercept.InterceptRecord {
	cwd, _ := os.Getwd()
	return intercept.InterceptRecord{
		Timestamp: start, AgentProtocol: proto.String(), WorkingDir: cwd,
		Command: input.Command, Outcome: outcome, RuleID: ruleID,
		Severity: severity, Reason: reason, SessionID: input.SessionID, ToolName: input.ToolName,
		EvalDurationUS: time.Since(start).Microseconds(),
	}
}

// makeRecordWithConfidence 构造带 confidence/matched_span 的记录(deny/ask 用)。
// 透传 proto 给 makeRecord;补 Confidence/MatchedSpan/ID(R3 新增字段)。
func makeRecordWithConfidence(input intercept.HookInput, proto intercept.AgentProtocol, outcome, ruleID, severity, reason, confidence, matchedSpan string, start time.Time) intercept.InterceptRecord {
	rec := makeRecord(input, proto, outcome, ruleID, severity, reason, start)
	rec.Confidence = confidence
	rec.MatchedSpan = matchedSpan
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
