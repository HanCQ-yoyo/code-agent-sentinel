package security

import (
	"testing"

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
