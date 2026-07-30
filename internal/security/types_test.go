package security

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFindingStartedAtSourcePathJSON(t *testing.T) {
	ts := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	f := Finding{RuleID: "r", StartedAt: ts, SourcePath: "/home/x/.claude/settings.json"}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["started_at"] == nil {
		t.Errorf("started_at missing in JSON: %s", b)
	}
	if m["source_path"] != "/home/x/.claude/settings.json" {
		t.Errorf("source_path mismatch: %v", m["source_path"])
	}
}
