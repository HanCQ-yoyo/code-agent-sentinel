// internal/intercept/hookio.go
package intercept

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrInputTooLarge 表示 stdin 超过 maxBytes 上限(dcg HookReadError::InputTooLarge)。
var ErrInputTooLarge = errors.New("intercept: input too large")

// HookInput 是 hook stdin 的宽松解析结果(Claude-only,全字段可选)。
// 参考 dcg HookInput(hook.rs:21-75):所有字段 Option,Claude 走 tool_input.command。
type HookInput struct {
	HookEventName string
	ToolName      string
	Command       string // tool_input.command
	Cwd           string
	SessionID     string
}

// Decision 是 hook 决策:allow(空 stdout)/ deny(JSON)/ ask(JSON,超时)。
type Decision int

const (
	DecisionAllow Decision = iota
	DecisionDeny
	DecisionAsk
)

// rawHookInput 是 stdin JSON 的原始结构(tool_input.command 是嵌套 string)。
type rawHookInput struct {
	HookEventName string `json:"hook_event_name"`
	ToolName      string `json:"tool_name"`
	ToolInput     struct {
		Command string `json:"command"`
	} `json:"tool_input"`
	Cwd       string `json:"cwd"`
	SessionID string `json:"session_id"`
}

// ParseHookInput 读 stdin(剥 UTF-8 BOM,带字节上限)解析为 HookInput。
// 参考 dcg read_hook_input(hook.rs:467-490)。解析失败由调用方 fail-open。
func ParseHookInput(r io.Reader, maxBytes int) (HookInput, error) {
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
			if sb.Len() > maxBytes {
				return HookInput{}, ErrInputTooLarge
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return HookInput{}, err
		}
	}
	s := strings.TrimPrefix(sb.String(), "\ufeff") // 剥 BOM(dcg hook.rs:484)
	var raw rawHookInput
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return HookInput{}, err
	}
	return HookInput{
		HookEventName: raw.HookEventName, ToolName: raw.ToolName,
		Command: raw.ToolInput.Command, Cwd: raw.Cwd, SessionID: raw.SessionID,
	}, nil
}

// hookOutput 是 deny/ask 的 stdout JSON 结构(dcg HookOutput,hook.rs:99-151)。
type hookOutput struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}
type hookSpecificOutput struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"` // "deny" / "ask"
	PermissionDecisionReason string `json:"permissionDecisionReason"`
	RuleID                   string `json:"ruleId,omitempty"`
	Severity                 string `json:"severity,omitempty"`
	Remediation              string `json:"remediation,omitempty"`
}

// WriteDecision 写 stdout 决策。allow → 空 stdout(fail-open);deny/ask → 一行 JSON。
// 参考 dcg main.rs deny 路径(hook.rs:1256-1274)。调用方负责 flush。
func WriteDecision(w io.Writer, dec Decision, reason, ruleID, severity, remediation string) {
	if dec == DecisionAllow {
		return // 空 stdout = allow
	}
	permission := "ask"
	if dec == DecisionDeny {
		permission = "deny"
	}
	out := hookOutput{HookSpecificOutput: hookSpecificOutput{
		HookEventName: "PreToolUse", PermissionDecision: permission,
		PermissionDecisionReason: reason, RuleID: ruleID, Severity: severity, Remediation: remediation,
	}}
	data, _ := json.Marshal(out)
	fmt.Fprintln(w, string(data)) // 一行 JSON + 换行
}
