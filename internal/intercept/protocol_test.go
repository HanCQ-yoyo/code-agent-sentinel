// internal/intercept/protocol_test.go
package intercept

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDetectProtocolCodexTurnID(t *testing.T) {
	if got := DetectProtocol("bash", "turn-1"); got != ProtoCodex {
		t.Fatalf("bash + turn_id 应判 Codex: got %v", got)
	}
	if got := DetectProtocol("Bash", "turn-1"); got != ProtoCodex {
		t.Fatalf("Bash + turn_id 应判 Codex: got %v", got)
	}
}

func TestDetectProtocolClaudeNoTurnID(t *testing.T) {
	if got := DetectProtocol("Bash", ""); got != ProtoClaude {
		t.Fatalf("Bash 无 turn_id 应判 Claude: got %v", got)
	}
}

func TestDetectProtocolPowerShellCodex(t *testing.T) {
	// PowerShell tool_name 无 turn_id 也判 Codex(dcg issue #125 Windows)
	if got := DetectProtocol("powershell", ""); got != ProtoCodex {
		t.Fatalf("powershell 应判 Codex: got %v", got)
	}
	if got := DetectProtocol("pwsh", ""); got != ProtoCodex {
		t.Fatalf("pwsh 应判 Codex: got %v", got)
	}
}

func TestWriteDecisionCodexDenyMinimal(t *testing.T) {
	var buf bytes.Buffer
	WriteDecision(&buf, ProtoCodex, DecisionDeny, "危险", "rule.x", "critical", "修复")
	var got map[string]map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("Codex deny 应合法 JSON: %v\n%s", err, buf.String())
	}
	hso := got["hookSpecificOutput"]
	if hso["permissionDecision"] != "deny" {
		t.Fatalf("应 deny: %v", hso["permissionDecision"])
	}
	// Codex 最小 payload:不应有 rule_id/severity/confidence/remediation/packId 扩展字段
	for _, k := range []string{"ruleId", "severity", "confidence", "remediation", "packId"} {
		if _, ok := hso[k]; ok {
			t.Fatalf("Codex 不应发扩展字段 %q(strict parser 会拒): %+v", k, hso)
		}
	}
}

func TestWriteDecisionCodexAskDegradesToDeny(t *testing.T) {
	var buf bytes.Buffer
	WriteDecision(&buf, ProtoCodex, DecisionAsk, "低置信度", "rule.x", "low", "")
	var got map[string]map[string]any
	json.Unmarshal(buf.Bytes(), &got)
	// Codex ask 退化为 deny
	if got["hookSpecificOutput"]["permissionDecision"] != "deny" {
		t.Fatalf("Codex ask 应退化为 deny: %v", got["hookSpecificOutput"]["permissionDecision"])
	}
	// reason 应标注低置信度
	reason, _ := got["hookSpecificOutput"]["permissionDecisionReason"].(string)
	if !strings.Contains(reason, "低置信度") {
		t.Fatalf("Codex ask 退化 reason 应标注低置信度: %q", reason)
	}
}

func TestWriteDecisionClaudeDenyFull(t *testing.T) {
	var buf bytes.Buffer
	WriteDecision(&buf, ProtoClaude, DecisionDeny, "危险", "rule.x", "critical", "修复")
	var got map[string]map[string]any
	json.Unmarshal(buf.Bytes(), &got)
	hso := got["hookSpecificOutput"]
	if hso["permissionDecision"] != "deny" {
		t.Fatalf("应 deny: %v", hso["permissionDecision"])
	}
	// Claude 发完整 payload(含扩展字段)
	if hso["ruleId"] != "rule.x" || hso["severity"] != "critical" {
		t.Fatalf("Claude 应发完整 payload: %+v", hso)
	}
}

func TestWriteDecisionAllowEmptyBoth(t *testing.T) {
	// 两协议 allow 都空 stdout
	for _, p := range []AgentProtocol{ProtoClaude, ProtoCodex} {
		var buf bytes.Buffer
		WriteDecision(&buf, p, DecisionAllow, "", "", "", "")
		if buf.Len() != 0 {
			t.Fatalf("协议 %v allow 应空 stdout: %q", p, buf.String())
		}
	}
}

func TestParseHookInputTurnID(t *testing.T) {
	stdin := `{"tool_name":"Bash","tool_input":{"command":"rm -rf /"},"turn_id":"turn-1"}`
	got, err := ParseHookInput(strings.NewReader(stdin), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if got.TurnID != "turn-1" {
		t.Fatalf("应解析 turn_id: got %q", got.TurnID)
	}
}
