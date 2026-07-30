package security

import (
	"testing"
	"time"

	"code-agent-sentinel/internal/security/findingstate"
)

func TestApplyFindingStateAccepted(t *testing.T) {
	states := &findingstate.States{Items: []findingstate.State{
		{Fingerprint: "fp1", Status: findingstate.StatusAccepted, Priority: "P2", Note: "ok"},
	}}
	f := Finding{RuleID: "r", Fingerprint: "fp1"}
	applyFindingState(&f, "fp1", states)
	if !f.Suppressed {
		t.Error("Suppressed should be true for accepted")
	}
	if f.Suppression != "state" {
		t.Errorf("Suppression = %q, want \"state\"", f.Suppression)
	}
	if f.Status != "accepted" {
		t.Errorf("Status = %q, want \"accepted\"", f.Status)
	}
	if f.Priority != "P2" {
		t.Errorf("Priority = %q, want P2", f.Priority)
	}
	if f.Note != "ok" {
		t.Errorf("Note = %q, want \"ok\"", f.Note)
	}
}

func TestApplyFindingStateNoMatchLeavesOpen(t *testing.T) {
	states := &findingstate.States{}
	f := Finding{RuleID: "r", Fingerprint: "fp1"}
	applyFindingState(&f, "fp1", states)
	if f.Suppressed {
		t.Error("Suppressed should be false when no match")
	}
	if f.Status != "open" {
		t.Errorf("Status = %q, want \"open\"", f.Status)
	}
}

func TestApplyFindingStateNilStatesSafe(t *testing.T) {
	f := Finding{RuleID: "r"}
	applyFindingState(&f, "fp1", nil) // 不 panic
	if f.Status != "open" {
		t.Errorf("Status = %q, want \"open\"", f.Status)
	}
}

func TestApplyFindingStateInProgressHalfWeight(t *testing.T) {
	// in_progress 应 Suppressed=false(仍可见)但 Status=in_progress(健康分 0.5)
	states := &findingstate.States{Items: []findingstate.State{
		{Fingerprint: "fp1", Status: findingstate.StatusInProgress},
	}}
	f := Finding{Fingerprint: "fp1"}
	applyFindingState(&f, "fp1", states)
	if f.Suppressed {
		t.Error("in_progress should not be Suppressed (visible)")
	}
	if f.Status != "in_progress" {
		t.Errorf("Status = %q, want in_progress", f.Status)
	}
}

func TestApplyFindingStateBatchAttachStateAndMeta(t *testing.T) {
	states := &findingstate.States{Items: []findingstate.State{{Fingerprint: "fp1", Status: findingstate.StatusResolved}}}
	ts := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	assetPath := func(assetID string) string {
		if assetID == "a1" {
			return "/home/x/.claude/settings.json"
		}
		return ""
	}
	findings := []Finding{
		{RuleID: "r1", Fingerprint: "fp1", AssetID: "a1"},
		{RuleID: "r2", Fingerprint: "fp2", AssetID: "a2"},
	}
	ApplyFindingStateBatch(findings, states, ts, assetPath)

	// fp1:resolved → suppressed=true + started_at + source_path
	if !findings[0].Suppressed {
		t.Errorf("findings[0] should be suppressed")
	}
	if findings[0].Status != "resolved" {
		t.Errorf("findings[0] status = %q, want resolved", findings[0].Status)
	}
	if !findings[0].StartedAt.Equal(ts) {
		t.Errorf("findings[0] StartedAt not attached")
	}
	if findings[0].SourcePath != "/home/x/.claude/settings.json" {
		t.Errorf("findings[0] SourcePath = %q", findings[0].SourcePath)
	}

	// fp2:无匹配 state → status open + started_at + source_path(空)
	if findings[1].Suppressed {
		t.Errorf("findings[1] should not be suppressed")
	}
	if findings[1].Status != "open" {
		t.Errorf("findings[1] status = %q, want open", findings[1].Status)
	}
	if !findings[1].StartedAt.Equal(ts) {
		t.Errorf("findings[1] StartedAt not attached")
	}
}

func TestApplyFindingStateBatchNilStatesSafe(t *testing.T) {
	ts := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	findings := []Finding{{RuleID: "r", Fingerprint: "fp"}}
	ApplyFindingStateBatch(findings, nil, ts, func(string) string { return "" })
	if findings[0].Status != "open" {
		t.Errorf("nil states should leave status open")
	}
	if !findings[0].StartedAt.Equal(ts) {
		t.Errorf("StartedAt should still attach with nil states")
	}
}
