// internal/intercept/protocol.go
package intercept

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// AgentProtocol 标识触发拦截的 AI 工具协议(决定 stdout 输出形态)。
// 评估管线(span/split/confidence/allowlist/语义关卡)对两协议完全相同,
// 唯一差异是输出 JSON 形态(见 WriteDecision)。
type AgentProtocol int

const (
	// ProtoClaude:完整 payload(含扩展字段 ruleId/severity/remediation),支持 ask。
	ProtoClaude AgentProtocol = iota
	// ProtoCodex:最小 payload(仅三字段),ask 退化为 deny。
	// Codex hook parser 严格(deny_unknown_fields,issue #183),多发扩展字段会被拒 → fail open。
	ProtoCodex
)

// String 返回协议名(InterceptRecord.AgentProtocol 用)。
func (p AgentProtocol) String() string {
	if p == ProtoCodex {
		return "codex"
	}
	return "claude"
}

// DetectProtocol 按 stdin 字段消歧(dcg hook.rs:580-620 核查)。
//   - shell tool(bash/Bash/launch-process)+ 非空 turn_id → Codex
//     (turn_id 是 codex-rs/hooks/schema.rs 标注的 "Codex extension",Claude 不发)
//   - PowerShell tool_name(powershell/pwsh)无 turn_id 也判 Codex
//     (dcg issue #125:Windows Codex 不总填 turn_id)
//   - 其余 → Claude(默认,fail-open)
//
// 大小写不敏感(strings.ToLower(strings.TrimSpace(toolName))),覆盖 Bash/bash。
func DetectProtocol(toolName, turnID string) AgentProtocol {
	tn := strings.ToLower(strings.TrimSpace(toolName))
	isShell := tn == "bash" || tn == "launch-process"
	isPowerShell := tn == "powershell" || tn == "pwsh"
	hasTurnID := strings.TrimSpace(turnID) != ""
	if isPowerShell {
		// PowerShell tool_name 只 Codex 发(Claude 的 shell tool 恒 "Bash")
		return ProtoCodex
	}
	if isShell && hasTurnID {
		return ProtoCodex
	}
	return ProtoClaude
}

// codexOutput 是 Codex 最小 payload(strict parser 不容扩展字段,不变量#5)。
// 仅 hookEventName / permissionDecision / permissionDecisionReason 三字段。
type codexOutput struct {
	HookSpecificOutput codexSpecific `json:"hookSpecificOutput"`
}

type codexSpecific struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
}

// WriteDecision 按 proto 写 stdout 决策(签名加 proto 首参,替换 R2 旧 WriteDecision)。
//   - Claude:deny/ask 发完整 JSON(含 ruleId/severity/remediation,供前端展示);allow 空 stdout。
//   - Codex:deny 发最小 JSON(仅三字段);ask 退化为 deny(reason 标注「低置信度,建议人工复核」);allow 空 stdout。
//
// 严格性:Codex 绝不多发扩展字段(不变量#5,防 strict parser 拒 → fail open)。
// exit 0 由调用方保证(hook 永远 exit 0,fail-open 铁律),本函数不调 os.Exit。
// 调用方负责 flush。
func WriteDecision(w io.Writer, proto AgentProtocol, dec Decision, reason, ruleID, severity, remediation string) {
	if dec == DecisionAllow {
		return // 两协议 allow 都空 stdout(fail-open 铁律)
	}

	if proto == ProtoCodex {
		// Codex:ask 退化为 deny,reason 标注低置信度
		finalReason := reason
		finalDecision := "deny"
		if dec == DecisionAsk {
			finalReason = "低置信度命中,建议人工复核: " + reason
		}
		out := codexOutput{HookSpecificOutput: codexSpecific{
			HookEventName:            "PreToolUse",
			PermissionDecision:       finalDecision,
			PermissionDecisionReason: finalReason,
		}}
		data, _ := json.Marshal(out)
		fmt.Fprintln(w, string(data)) // 一行 JSON + 换行
		return
	}

	// Claude:发完整 payload(含扩展字段,供前端展示),复用 R2 hookio.go 的 hookOutput 结构。
	permission := "ask"
	if dec == DecisionDeny {
		permission = "deny"
	}
	out := hookOutput{HookSpecificOutput: hookSpecificOutput{
		HookEventName:            "PreToolUse",
		PermissionDecision:       permission,
		PermissionDecisionReason: reason,
		RuleID:                   ruleID,
		Severity:                 severity,
		Remediation:              remediation,
	}}
	data, _ := json.Marshal(out)
	fmt.Fprintln(w, string(data)) // 一行 JSON + 换行
}
