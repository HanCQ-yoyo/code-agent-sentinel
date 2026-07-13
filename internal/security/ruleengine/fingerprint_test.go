package ruleengine

import (
	"testing"
)

// ── brief 给定的 3 个测试 ──

func TestFingerprintStable(t *testing.T) {
	r := mustRule(t, "permissions", map[string]any{"field": "allow", "op": "contains", "value": "Bash(*)"})
	fp1 := Fingerprint(r, "asset-1")
	fp2 := Fingerprint(r, "asset-1") // 同规则同资产
	if fp1 != fp2 {
		t.Fatal("same rule+asset must have stable fingerprint")
	}
}

func TestFingerprintIgnoresHitContent(t *testing.T) {
	// Fingerprint 仅取 (rule, assetID),签名里没有 content 参数,
	// 故指纹不可能依赖命中证据。直接断言此属性:同规则同资产两次调用稳定,
	// 且与"该资产内容是否变化"无关(Fingerprint 根本看不到内容)。
	r := mustRule(t, "permissions", map[string]any{"field": "allow", "op": "contains", "value": "Bash(*)"})
	fp := Fingerprint(r, "asset-1")
	if len(fp) != 64 {
		t.Fatalf("want sha256 hex 64 chars, got %d", len(fp))
	}
	// 稳定性:同 (rule, assetID) 再算一次必相等(指纹不含命中证据,纯函数)。
	if fp2 := Fingerprint(r, "asset-1"); fp2 != fp {
		t.Fatalf("Fingerprint 必须稳定且不依赖命中内容: fp1=%s fp2=%s", fp, fp2)
	}
}

func TestFingerprintDiffersByRule(t *testing.T) {
	r1 := mustRule(t, "permissions", map[string]any{"field": "allow", "op": "contains", "value": "Bash(*)"})
	r2 := mustRule(t, "permissions", map[string]any{"field": "allow", "op": "contains", "value": "Edit(**)"})
	if Fingerprint(r1, "a") == Fingerprint(r2, "a") {
		t.Fatal("different rule value must differ")
	}
}

// ── 边缘场景测试 ──

// 不同 assetID → 不同指纹(同规则)。
func TestFingerprintDiffersByAsset(t *testing.T) {
	r := mustRule(t, "permissions", map[string]any{"field": "allow", "op": "contains", "value": "Bash(*)"})
	if Fingerprint(r, "asset-1") == Fingerprint(r, "asset-2") {
		t.Fatal("different assetID must produce different fingerprint")
	}
}

// within op:两个规则 value 数组顺序不同 → 指纹相同(排序后规范化)。
// 这是 within 锚点的关键正确性属性。
func TestFingerprintWithinOrderStable(t *testing.T) {
	r1 := mustRule(t, "permissions", map[string]any{
		"field": "allow", "op": "within",
		"value": []any{"Bash(*)", "Read(/tmp)", "Edit(**)"},
	})
	r2 := mustRule(t, "permissions", map[string]any{
		"field": "allow", "op": "within",
		"value": []any{"Edit(**)", "Bash(*)", "Read(/tmp)"},
	})
	fp1 := Fingerprint(r1, "asset-1")
	fp2 := Fingerprint(r2, "asset-1")
	if fp1 != fp2 {
		t.Fatalf("within with same elements different order must have same fingerprint:\n  fp1=%s\n  fp2=%s", fp1, fp2)
	}
	if len(fp1) != 64 {
		t.Fatalf("want 64 chars, got %d", len(fp1))
	}
}

// exists op:锚点为空串,指纹仍为 64 字符且稳定。
func TestFingerprintExistsEmptyAnchor(t *testing.T) {
	r := mustRule(t, "settings", map[string]any{"field": "hooks", "op": "exists"})
	fp1 := Fingerprint(r, "asset-1")
	fp2 := Fingerprint(r, "asset-1")
	if fp1 != fp2 {
		t.Fatal("exists fingerprint must be stable")
	}
	if len(fp1) != 64 {
		t.Fatalf("want 64 chars, got %d", len(fp1))
	}
}

// 布尔节点(and):指纹取首个子节点锚点,稳定且 64 字符。
func TestFingerprintAndNode(t *testing.T) {
	r := mustRule(t, "permissions", map[string]any{
		"and": []any{
			map[string]any{"field": "allow", "op": "contains", "value": "Bash(*)"},
			map[string]any{"field": "deny", "op": "exists"},
		},
	})
	fp1 := Fingerprint(r, "asset-1")
	fp2 := Fingerprint(r, "asset-1")
	if fp1 != fp2 {
		t.Fatal("and-node fingerprint must be stable")
	}
	if len(fp1) != 64 {
		t.Fatalf("want 64 chars, got %d", len(fp1))
	}
	// and 的首个子节点是 contains Bash(*),与单叶子 contains Bash(*) 规则锚点相同
	leaf := mustRule(t, "permissions", map[string]any{"field": "allow", "op": "contains", "value": "Bash(*)"})
	if Fingerprint(leaf, "asset-1") != fp1 {
		t.Fatal("and-node should derive anchor from first child")
	}
}

// 布尔节点(or):指纹取首个子节点锚点。
func TestFingerprintOrNode(t *testing.T) {
	r := mustRule(t, "permissions", map[string]any{
		"or": []any{
			map[string]any{"field": "allow", "op": "regex_match", "value": "Bash\\(.*\\)"},
			map[string]any{"field": "deny", "op": "exists"},
		},
	})
	fp := Fingerprint(r, "asset-1")
	if len(fp) != 64 {
		t.Fatalf("want 64 chars, got %d", len(fp))
	}
	// or 的首个子节点是 regex_match,锚点应等于该单叶子规则
	leaf := mustRule(t, "permissions", map[string]any{"field": "allow", "op": "regex_match", "value": "Bash\\(.*\\)"})
	if Fingerprint(leaf, "asset-1") != fp {
		t.Fatal("or-node should derive anchor from first child")
	}
}

// 布尔节点(not):指纹取其唯一子节点锚点。
func TestFingerprintNotNode(t *testing.T) {
	r := mustRule(t, "permissions", map[string]any{
		"not": map[string]any{"field": "allow", "op": "eq", "value": "safe"},
	})
	fp := Fingerprint(r, "asset-1")
	if len(fp) != 64 {
		t.Fatalf("want 64 chars, got %d", len(fp))
	}
	leaf := mustRule(t, "permissions", map[string]any{"field": "allow", "op": "eq", "value": "safe"})
	if Fingerprint(leaf, "asset-1") != fp {
		t.Fatal("not-node should derive anchor from its child")
	}
}

// 禁用规则(Match.raw 为 nil)不 panic,指纹仍为 64 字符。
func TestFingerprintDisabledRule(t *testing.T) {
	r := Rule{ID: "test-rule", Severity: "high", AssetType: "permissions"}
	fp := Fingerprint(r, "asset-1")
	if len(fp) != 64 {
		t.Fatalf("want 64 chars for disabled rule, got %d", len(fp))
	}
}

// not_within 与 within 同样排序规范化。
func TestFingerprintNotWithinOrderStable(t *testing.T) {
	r1 := mustRule(t, "permissions", map[string]any{
		"field": "allow", "op": "not_within",
		"value": []any{"a", "b", "c"},
	})
	r2 := mustRule(t, "permissions", map[string]any{
		"field": "allow", "op": "not_within",
		"value": []any{"c", "a", "b"},
	})
	if Fingerprint(r1, "x") != Fingerprint(r2, "x") {
		t.Fatal("not_within with same elements different order must have same fingerprint")
	}
}
