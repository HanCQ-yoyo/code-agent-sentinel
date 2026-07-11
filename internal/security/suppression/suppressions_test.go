package suppression

import (
	"testing"
)

// ── 三档匹配测试 ──

func TestSuppressionsMatchFingerprintTier(t *testing.T) {
	s := &Suppressions{Items: []Item{
		{Fingerprint: "fp1", Reason: "r1"},
	}}
	suppressed, reason := s.Match("any-rule", "any-asset", "fp1")
	if !suppressed || reason != "r1" {
		t.Fatalf("指纹档: suppressed=%v reason=%s", suppressed, reason)
	}
}

func TestSuppressionsMatchRuleAndAssetTier(t *testing.T) {
	s := &Suppressions{Items: []Item{
		{RuleID: "injection.x", AssetID: "a1", Reason: "r2"},
	}}
	// 精确匹配
	suppressed, reason := s.Match("injection.x", "a1", "nope")
	if !suppressed || reason != "r2" {
		t.Fatalf("rule+asset 档: suppressed=%v reason=%s", suppressed, reason)
	}
	// asset 不匹配 → 不命中
	suppressed, _ = s.Match("injection.x", "a2", "nope")
	if suppressed {
		t.Fatal("rule+asset 档: asset 不匹配不应命中")
	}
	// rule 不匹配 → 不命中
	suppressed, _ = s.Match("injection.y", "a1", "nope")
	if suppressed {
		t.Fatal("rule+asset 档: rule 不匹配不应命中")
	}
}

func TestSuppressionsMatchRuleGlobalTier(t *testing.T) {
	s := &Suppressions{Items: []Item{
		{RuleID: "baseline.y", Reason: "r3"}, // 全局
	}}
	suppressed, reason := s.Match("baseline.y", "any-asset", "nope")
	if !suppressed || reason != "r3" {
		t.Fatalf("rule 全局档: suppressed=%v reason=%s", suppressed, reason)
	}
	// rule 不匹配 → 不命中
	suppressed, _ = s.Match("baseline.z", "any-asset", "nope")
	if suppressed {
		t.Fatal("rule 全局档: rule 不匹配不应命中")
	}
}

// ── 优先级测试:指纹 > rule+asset > rule 全局 ──

func TestSuppressionsPriorityFingerprintOverRuleAsset(t *testing.T) {
	s := &Suppressions{Items: []Item{
		{RuleID: "rule.x", AssetID: "a1", Reason: "rule+asset"},
		{Fingerprint: "fp1", Reason: "fingerprint"},
	}}
	suppressed, reason := s.Match("rule.x", "a1", "fp1")
	if !suppressed || reason != "fingerprint" {
		t.Fatalf("优先级: 指纹应优先于 rule+asset, got reason=%s", reason)
	}
}

func TestSuppressionsPriorityRuleAssetOverRuleGlobal(t *testing.T) {
	s := &Suppressions{Items: []Item{
		{RuleID: "rule.x", Reason: "rule-global"},
		{RuleID: "rule.x", AssetID: "a1", Reason: "rule+asset"},
	}}
	suppressed, reason := s.Match("rule.x", "a1", "nope")
	if !suppressed || reason != "rule+asset" {
		t.Fatalf("优先级: rule+asset 应优先于 rule 全局, got reason=%s", reason)
	}
}

func TestSuppressionsPriorityFingerprintOverAll(t *testing.T) {
	// 三档都匹配,指纹应赢
	s := &Suppressions{Items: []Item{
		{RuleID: "rule.x", AssetID: "a1", Reason: "rule+asset"},
		{RuleID: "rule.x", Reason: "rule-global"},
		{Fingerprint: "fp1", Reason: "fingerprint"},
	}}
	suppressed, reason := s.Match("rule.x", "a1", "fp1")
	if !suppressed || reason != "fingerprint" {
		t.Fatalf("优先级: 指纹应优先于所有, got reason=%s", reason)
	}
}

// ── 无匹配 ──

func TestSuppressionsNoMatch(t *testing.T) {
	s := &Suppressions{Items: []Item{
		{Fingerprint: "fp1", Reason: "r1"},
		{RuleID: "rule.x", AssetID: "a1", Reason: "r2"},
		{RuleID: "rule.y", Reason: "r3"},
	}}
	suppressed, _ := s.Match("unknown", "unknown", "unknown")
	if suppressed {
		t.Fatal("无匹配时应返回 false")
	}
}

// ── nil 安全 ──

func TestSuppressionsMatchNilSafe(t *testing.T) {
	var s *Suppressions
	suppressed, _ := s.Match("rule", "asset", "fp")
	if suppressed {
		t.Fatal("nil Suppressions 的 Match 应返回 false")
	}
}

// ── Load/Save 往返测试 ──

func TestSuppressionsSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/suppressions.yaml"

	original := &Suppressions{Items: []Item{
		{Fingerprint: "fp1", Reason: "r1"},
		{RuleID: "injection.x", AssetID: "a1", Reason: "r2"},
		{RuleID: "baseline.y", Reason: "r3"},
	}}
	if err := original.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadSuppressions(path)
	if err != nil {
		t.Fatalf("LoadSuppressions: %v", err)
	}
	if len(loaded.Items) != 3 {
		t.Fatalf("往返后 Items 数: got %d, want 3", len(loaded.Items))
	}
	// 验证各档可匹配
	suppressed, reason := loaded.Match("any", "any", "fp1")
	if !suppressed || reason != "r1" {
		t.Fatalf("指纹档往返后: suppressed=%v reason=%s", suppressed, reason)
	}
	suppressed, reason = loaded.Match("injection.x", "a1", "nope")
	if !suppressed || reason != "r2" {
		t.Fatalf("rule+asset 档往返后: suppressed=%v reason=%s", suppressed, reason)
	}
	suppressed, reason = loaded.Match("baseline.y", "any", "nope")
	if !suppressed || reason != "r3" {
		t.Fatalf("rule 全局档往返后: suppressed=%v reason=%s", suppressed, reason)
	}
}

// ── LoadSuppressions 文件不存在返回 (nil, nil) ──

func TestLoadSuppressionsMissingFileIsEmpty(t *testing.T) {
	s, err := LoadSuppressions("/nonexistent/path/suppressions.yaml")
	if err != nil {
		t.Fatalf("文件不存在应返回 (nil, nil), got err: %v", err)
	}
	if s != nil {
		t.Fatalf("文件不存在应返回 nil, got %+v", s)
	}
}
