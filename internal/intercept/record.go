// internal/intercept/record.go
package intercept

import (
	"encoding/json"
	"time"
)

// InterceptRecord 是单条命令的拦截决策事件。存 sentinel.db 的 intercept_records 表。
// 精简:砍 exit_code/parent_command_id/hostname/allowlist_layer/bypass_code
// (v1 纯 deny 无 allowlist)。加 ToolName(UI 筛选用)。
// 命名空间:AgentProtocol 由 R3 协议探测动态填(claude/codex);R2 写死 "claude"。
type InterceptRecord struct {
	ID             string    `json:"id"`
	Timestamp      time.Time `json:"timestamp"`
	AgentProtocol  string    `json:"agent_protocol"`
	WorkingDir     string    `json:"working_dir"`
	Command        string    `json:"command"`
	Outcome        string    `json:"outcome"` // allow / deny / warn / ask
	RuleID         string    `json:"rule_id,omitempty"`
	PackID         string    `json:"pack_id,omitempty"`
	Severity       string    `json:"severity,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	EvalDurationUS int64     `json:"eval_duration_us"`
	SessionID      string    `json:"session_id,omitempty"`
	ToolName       string    `json:"tool_name,omitempty"`
	// Stage R3:命中置信度(high/low/unknown)。allow 时空。
	Confidence string `json:"confidence,omitempty"`
	// Stage R3:命中片段文本(链式拆分后定位用,如 `&& rm -rf /` 的 rm -rf /)。allow 时空。
	MatchedSpan string `json:"matched_span,omitempty"`
}

func (r InterceptRecord) MarshalJSON() ([]byte, error) {
	type alias InterceptRecord // 防 MarshalJSON 递归
	return json.Marshal(alias(r))
}

func (r *InterceptRecord) UnmarshalJSON(data []byte) error {
	type alias InterceptRecord
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*r = InterceptRecord(a)
	return nil
}
