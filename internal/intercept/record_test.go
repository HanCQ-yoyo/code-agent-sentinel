// internal/intercept/record_test.go
package intercept

import "testing"

func TestInterceptRecordJSONRoundTrip(t *testing.T) {
	rec := InterceptRecord{
		ID: "20260728-abc123", AgentProtocol: "claude", Outcome: "deny",
		Command: "rm -rf /", RuleID: "destructive.filesystem.rm-rf-root",
		Severity: "critical", Reason: "递归删除根目录", EvalDurationUS: 1500,
		WorkingDir: "/home/user/proj", SessionID: "ses-1", ToolName: "Bash",
	}
	data, err := rec.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var got InterceptRecord
	if err := got.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if got.ID != rec.ID || got.Outcome != "deny" || got.RuleID != rec.RuleID || got.EvalDurationUS != 1500 {
		t.Fatalf("round-trip 丢失字段: %+v", got)
	}
}
