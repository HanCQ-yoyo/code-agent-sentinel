// internal/intercept/hookio.go
package intercept

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// ErrInputTooLarge 表示 stdin 超过 maxBytes 上限。
var ErrInputTooLarge = errors.New("intercept: input too large")

// HookInput 是 hook stdin 的宽松解析结果(Claude/Codex 通用,全字段可选)。
// 所有字段 Option;Claude 走 tool_input.command,Codex 同(Codex schema 复用 tool_input.command)。
// TurnID 非空是 Codex 协议的强信号(Claude 不发 turn_id),供 DetectProtocol 消歧。
type HookInput struct {
	HookEventName string
	ToolName      string
	Command       string // tool_input.command
	Cwd           string
	SessionID     string
	TurnID        string // codex-rs 扩展字段(Claude 不发),用于协议探测
}

// Decision 是 hook 决策:allow(空 stdout)/ deny(JSON)/ ask(JSON,超时)。
type Decision int

const (
	DecisionAllow Decision = iota
	DecisionDeny
	DecisionAsk
)

// rawHookInput 是 stdin JSON 的原始结构(tool_input.command 是嵌套 string)。
// turn_id 是 codex-rs 扩展字段(Claude 不发),用于协议探测。
type rawHookInput struct {
	HookEventName string `json:"hook_event_name"`
	ToolName      string `json:"tool_name"`
	ToolInput     struct {
		Command string `json:"command"`
	} `json:"tool_input"`
	Cwd       string `json:"cwd"`
	SessionID string `json:"session_id"`
	TurnID    string `json:"turn_id"`
}

// ParseHookInput 读 stdin(剥 UTF-8 BOM,带字节上限)解析为 HookInput。
// 解析失败由调用方 fail-open。
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
	s := strings.TrimPrefix(sb.String(), "\ufeff") // 剥 BOM
	var raw rawHookInput
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return HookInput{}, err
	}
	return HookInput{
		HookEventName: raw.HookEventName, ToolName: raw.ToolName,
		Command: raw.ToolInput.Command, Cwd: raw.Cwd, SessionID: raw.SessionID,
		TurnID: raw.TurnID,
	}, nil
}

// hookOutput 是 Claude deny/ask 的 stdout JSON 结构(完整 payload,含扩展字段)。
// Codex 路径不使用此结构(见 protocol.go 的 codexOutput,仅三字段)。
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

// WriteDecision 已迁至 protocol.go(签名加 proto AgentProtocol 首参,支持 Claude/Codex 分支)。
// hookOutput/hookSpecificOutput 留在此文件供 protocol.go 的 Claude 路径复用(同包可见)。
