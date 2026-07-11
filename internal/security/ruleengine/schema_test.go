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
