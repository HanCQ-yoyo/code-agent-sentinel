package ruleengine

import "testing"

func TestOpConstants(t *testing.T) {
	// 11 个 op,6 类
	ops := []string{OpExists, OpNotExists, OpEq, OpNotEquals, OpContains, OpNotContains,
		OpRegexMatch, OpNotRegexMatch, OpKeyMatches, OpWithin, OpNotWithin}
	if len(ops) != 11 {
		t.Fatalf("want 11 ops, got %d", len(ops))
	}
	seen := map[string]bool{}
	for _, o := range ops {
		if seen[o] {
			t.Errorf("dup op %s", o)
		}
		seen[o] = true
	}
	if !validOp(OpContains) {
		t.Error("contains should be valid")
	}
	if validOp("bogus") {
		t.Error("bogus should be invalid")
	}
}

// TestComboRuleYAMLDecode 验证 parseRuleYAML 能把 combo_rules 段解析为 []ComboRule,
// 并正确填充 ID/Severity/Requires/AssetType。锁 Task 8 的 combo 加载契约。
func TestComboRuleYAMLDecode(t *testing.T) {
	data := []byte(`
combo_rules:
  - id: combo.test
    severity: critical
    description: "test combo"
    requires:
      - asset_type: settings
        match: { field: skip_dangerous, op: eq, value: "true" }
      - asset_type: permissions
        match: { field: allow, op: contains, value: "Bash(*)" }
`)
	rules, combos, errs := parseRuleYAML(data, "test")
	if len(errs) > 0 {
		t.Fatalf("parse errs: %v", errs)
	}
	if len(rules) != 0 || len(combos) != 1 {
		t.Fatalf("got %d rules, %d combos, want 0 rules 1 combo", len(rules), len(combos))
	}
	c := combos[0]
	if c.ID != "combo.test" || c.Severity != "critical" {
		t.Fatalf("combo = %+v", c)
	}
	if len(c.Requires) != 2 {
		t.Fatalf("requires len = %d, want 2", len(c.Requires))
	}
	if c.Requires[0].AssetType != "settings" {
		t.Fatalf("require[0] asset_type = %q", c.Requires[0].AssetType)
	}
}

func TestMatchNodeRawRoundTrip(t *testing.T) {
	// 叶子节点 map{field,op,value}
	leaf := map[string]any{"field": "command", "op": "contains", "value": "rm -rf"}
	mn := NewMatchNode(leaf)
	got := mn.Raw()
	if got["field"] != "command" || got["op"] != "contains" || got["value"] != "rm -rf" {
		t.Fatalf("Raw() = %#v, want leaf map preserved", got)
	}

	// 布尔节点 map{and: [...]}
	and := map[string]any{"and": []any{
		map[string]any{"field": "command", "op": "contains", "value": "rm"},
		map[string]any{"field": "command", "op": "contains", "value": "rf"},
	}}
	mn2 := NewMatchNode(and)
	got2 := mn2.Raw()
	if _, ok := got2["and"]; !ok {
		t.Fatalf("Raw() = %#v, want 'and' key preserved", got2)
	}
}

func TestNewMatchNodeFromNilIsEmpty(t *testing.T) {
	mn := NewMatchNode(nil)
	if mn.Raw() != nil {
		t.Fatalf("Raw() = %#v, want nil for nil input", mn.Raw())
	}
}
