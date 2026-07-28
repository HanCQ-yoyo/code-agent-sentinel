// internal/intercept/hookio_test.go
package intercept

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseHookInputClaude(t *testing.T) {
	stdin := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"rm -rf /"},"cwd":"/home/u/p","session_id":"ses-1"}`
	got, err := ParseHookInput(strings.NewReader(stdin), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if got.Command != "rm -rf /" || got.ToolName != "Bash" || got.Cwd != "/home/u/p" || got.SessionID != "ses-1" {
		t.Fatalf("解析不对: %+v", got)
	}
}

func TestParseHookInputBOM(t *testing.T) {
	stdin := "\ufeff" + `{"tool_name":"Bash","tool_input":{"command":"git status"}}`
	got, err := ParseHookInput(strings.NewReader(stdin), 1<<20)
	if err != nil {
		t.Fatalf("BOM 应被剥: %v", err)
	}
	if got.Command != "git status" {
		t.Fatalf("命令不对: %q", got.Command)
	}
}

func TestParseHookInputTooLarge(t *testing.T) {
	big := strings.Repeat("x", 100)
	_, err := ParseHookInput(strings.NewReader(big), 10)
	if err != ErrInputTooLarge {
		t.Fatalf("超长应 ErrInputTooLarge, got %v", err)
	}
}

func TestWriteDecisionDeny(t *testing.T) {
	var buf bytes.Buffer
	WriteDecision(&buf, DecisionDeny, "rm -rf / 危险", "destructive.filesystem.rm-rf-root", "critical", "先 git stash")
	var got map[string]map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("deny 输出应是合法 JSON: %v\n%s", err, buf.String())
	}
	hso := got["hookSpecificOutput"]
	if hso["permissionDecision"] != "deny" || hso["hookEventName"] != "PreToolUse" {
		t.Fatalf("deny 字段不对: %+v", hso)
	}
	if hso["permissionDecisionReason"] != "rm -rf / 危险" {
		t.Fatalf("reason 不对: %v", hso["permissionDecisionReason"])
	}
}

func TestWriteDecisionAllowEmpty(t *testing.T) {
	var buf bytes.Buffer
	WriteDecision(&buf, DecisionAllow, "", "", "", "")
	if buf.Len() != 0 {
		t.Fatalf("allow 应输出空 stdout, got %q", buf.String())
	}
}
