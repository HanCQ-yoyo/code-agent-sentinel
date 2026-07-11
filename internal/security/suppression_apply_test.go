package security

import (
	"testing"

	"code-agent-sentinel/internal/security/suppression"
)

// TestBaselineSuppressesKnownFingerprint: baseline 命中 → Suppression="baseline", Reason=""
func TestBaselineSuppressesKnownFingerprint(t *testing.T) {
	bs := &suppression.BaselineSet{Fingerprints: map[string]bool{"fp1": true}}
	f := Finding{RuleID: "rule.x", AssetID: "a1"}

	applySuppression(&f, "fp1", bs, nil)

	if !f.Suppressed {
		t.Fatal("baseline 命中应设置 Suppressed=true")
	}
	if f.Suppression != "baseline" {
		t.Fatalf("Suppression: got %q, want %q", f.Suppression, "baseline")
	}
	if f.Reason != "" {
		t.Fatalf("baseline 命中 Reason 应为空, got %q", f.Reason)
	}
}

// TestSuppressionsThreeTiers: 行内三档豁免
func TestSuppressionsThreeTiers(t *testing.T) {
	s := &suppression.Suppressions{Items: []suppression.Item{
		{Fingerprint: "fp1", Reason: "r1"},
		{RuleID: "injection.x", AssetID: "a1", Reason: "r2"},
		{RuleID: "baseline.y", Reason: "r3"}, // 全局
	}}

	// 精准指纹
	f := Finding{RuleID: "z", AssetID: "z"}
	applySuppression(&f, "fp1", nil, s)
	if !f.Suppressed || f.Suppression != "inline" || f.Reason != "r1" {
		t.Fatalf("指纹档: suppressed=%v suppression=%q reason=%q", f.Suppressed, f.Suppression, f.Reason)
	}

	// rule+asset
	f = Finding{RuleID: "injection.x", AssetID: "a1"}
	applySuppression(&f, "nope", nil, s)
	if !f.Suppressed || f.Suppression != "inline" || f.Reason != "r2" {
		t.Fatalf("rule+asset 档: suppressed=%v suppression=%q reason=%q", f.Suppressed, f.Suppression, f.Reason)
	}

	// rule 全局
	f = Finding{RuleID: "baseline.y", AssetID: "any"}
	applySuppression(&f, "nope", nil, s)
	if !f.Suppressed || f.Suppression != "inline" || f.Reason != "r3" {
		t.Fatalf("rule 全局档: suppressed=%v suppression=%q reason=%q", f.Suppressed, f.Suppression, f.Reason)
	}
}

// TestApplySuppressionNoMatch: 无匹配 → finding 不变
func TestApplySuppressionNoMatch(t *testing.T) {
	bs := &suppression.BaselineSet{Fingerprints: map[string]bool{"known": true}}
	s := &suppression.Suppressions{Items: []suppression.Item{
		{Fingerprint: "fp1", Reason: "r1"},
		{RuleID: "rule.x", AssetID: "a1", Reason: "r2"},
	}}

	f := Finding{RuleID: "unknown", AssetID: "unknown"}
	applySuppression(&f, "unknown-fp", bs, s)

	if f.Suppressed {
		t.Fatal("无匹配时 Suppressed 应保持 false")
	}
	if f.Suppression != "" {
		t.Fatalf("无匹配时 Suppression 应为空, got %q", f.Suppression)
	}
	if f.Reason != "" {
		t.Fatalf("无匹配时 Reason 应为空, got %q", f.Reason)
	}
}

// TestApplySuppressionNilBoth: baseline=nil + supprs=nil → 不 panic, finding 不变
func TestApplySuppressionNilBoth(t *testing.T) {
	f := Finding{RuleID: "rule.x", AssetID: "a1"}

	applySuppression(&f, "fp1", nil, nil)

	if f.Suppressed {
		t.Fatal("nil baseline + nil supprs 时 Suppressed 应保持 false")
	}
	if f.Suppression != "" || f.Reason != "" {
		t.Fatalf("nil 时 Suppression/Reason 应为空, got %q/%q", f.Suppression, f.Reason)
	}
}

// TestApplySuppressionBaselineTakesPrecedenceOverInline: baseline 优先于 inline
func TestApplySuppressionBaselineTakesPrecedenceOverInline(t *testing.T) {
	bs := &suppression.BaselineSet{Fingerprints: map[string]bool{"fp1": true}}
	s := &suppression.Suppressions{Items: []suppression.Item{
		{Fingerprint: "fp1", Reason: "inline-reason"},
	}}

	f := Finding{RuleID: "rule.x", AssetID: "a1"}
	applySuppression(&f, "fp1", bs, s)

	if !f.Suppressed || f.Suppression != "baseline" {
		t.Fatalf("baseline 应优先于 inline: suppressed=%v suppression=%q", f.Suppressed, f.Suppression)
	}
	if f.Reason != "" {
		t.Fatalf("baseline 命中 Reason 应为空, got %q", f.Reason)
	}
}
