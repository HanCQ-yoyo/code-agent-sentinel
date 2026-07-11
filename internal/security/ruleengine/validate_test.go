package ruleengine

import "testing"

// ── brief 给定的 4 个测试 ──

func TestValidateValidRule(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "high", AssetType: "permissions",
		Match: MatchNode{raw: map[string]any{"field": "allow", "op": "contains", "value": "Bash(*)"}}}}
	valid, errs := Validate(rules)
	if len(errs) != 0 || len(valid) != 1 {
		t.Fatalf("want 1 valid 0 err, got %d %v", len(valid), errs)
	}
}

func TestValidateMissingMatchIsDisabled(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "high", AssetType: "settings"}} // 无 match
	valid, errs := Validate(rules)
	if len(errs) != 0 || len(valid) != 1 {
		t.Fatalf("disabled rule should be valid, got valid=%d errs=%v", len(valid), errs)
	}
}

func TestValidateEmptyAndFails(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "high", AssetType: "settings",
		Match: MatchNode{raw: map[string]any{"and": []any{}}}}}
	_, errs := Validate(rules)
	if len(errs) == 0 {
		t.Fatal("empty and should fail")
	}
}

func TestValidateBadRegexFails(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "high", AssetType: "skill",
		Match: MatchNode{raw: map[string]any{"field": "content", "op": "regex_match", "value": "(?P<bad"}}}}
	_, errs := Validate(rules)
	if len(errs) == 0 {
		t.Fatal("bad regex should fail")
	}
}

// ── 补全 brief Step 1 的 8 个覆盖点 ──

// 2. 缺 id 报错
func TestValidateMissingIDFails(t *testing.T) {
	rules := []Rule{{Severity: "high", AssetType: "settings",
		Match: MatchNode{raw: map[string]any{"field": "x", "op": "exists"}}}}
	_, errs := Validate(rules)
	if len(errs) == 0 {
		t.Fatal("missing id should fail")
	}
}

// 3. op 不在枚举报错
func TestValidateBadOpFails(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "high", AssetType: "settings",
		Match: MatchNode{raw: map[string]any{"field": "x", "op": "bogus_op", "value": "y"}}}}
	_, errs := Validate(rules)
	if len(errs) == 0 {
		t.Fatal("invalid op should fail")
	}
}

// 6. not 多子节点(list)报错
func TestValidateNotMultipleChildrenFails(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "high", AssetType: "settings",
		Match: MatchNode{raw: map[string]any{"not": []any{
			map[string]any{"field": "a", "op": "exists"},
			map[string]any{"field": "b", "op": "exists"},
		}}}}}
	_, errs := Validate(rules)
	if len(errs) == 0 {
		t.Fatal("not with list (multiple children) should fail")
	}
}

// 8. asset_type 非法报错
func TestValidateBadAssetTypeFails(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "high", AssetType: "bogus_type",
		Match: MatchNode{raw: map[string]any{"field": "x", "op": "exists"}}}}
	_, errs := Validate(rules)
	if len(errs) == 0 {
		t.Fatal("invalid asset_type should fail")
	}
}

// ── 额外覆盖 ──

// severity 非法报错
func TestValidateBadSeverityFails(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "urgent", AssetType: "settings",
		Match: MatchNode{raw: map[string]any{"field": "x", "op": "exists"}}}}
	_, errs := Validate(rules)
	if len(errs) == 0 {
		t.Fatal("invalid severity should fail")
	}
}

// 嵌套 and/or 合法
func TestValidateNestedAndOr(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "medium", AssetType: "settings",
		Match: MatchNode{raw: map[string]any{"and": []any{
			map[string]any{"field": "a", "op": "exists"},
			map[string]any{"or": []any{
				map[string]any{"field": "b", "op": "eq", "value": "1"},
				map[string]any{"field": "c", "op": "contains", "value": "x"},
			}},
		}}}}}
	valid, errs := Validate(rules)
	if len(errs) != 0 || len(valid) != 1 {
		t.Fatalf("nested and/or should be valid, got errs=%v", errs)
	}
}

// not 单 map 子节点合法
func TestValidateNotWithMap(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "low", AssetType: "settings",
		Match: MatchNode{raw: map[string]any{"not": map[string]any{
			"field": "a", "op": "exists",
		}}}}}
	valid, errs := Validate(rules)
	if len(errs) != 0 || len(valid) != 1 {
		t.Fatalf("not with single map child should be valid, got errs=%v", errs)
	}
}

// within 要求非空数组
func TestValidateWithinRequiresArray(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "high", AssetType: "settings",
		Match: MatchNode{raw: map[string]any{"field": "x", "op": "within", "value": "not_array"}}}}
	_, errs := Validate(rules)
	if len(errs) == 0 {
		t.Fatal("within with non-array value should fail")
	}
}

// within 空数组也报错
func TestValidateWithinEmptyArrayFails(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "high", AssetType: "settings",
		Match: MatchNode{raw: map[string]any{"field": "x", "op": "within", "value": []any{}}}}}
	_, errs := Validate(rules)
	if len(errs) == 0 {
		t.Fatal("within with empty array should fail")
	}
}

// 正则编译成功后存入 rule.regexes
func TestValidateCompilesRegex(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "high", AssetType: "skill",
		Match: MatchNode{raw: map[string]any{"field": "content", "op": "regex_match", "value": "bash"}}}}
	valid, errs := Validate(rules)
	if len(errs) != 0 || len(valid) != 1 {
		t.Fatalf("want 1 valid 0 err, got %d %v", len(valid), errs)
	}
	if valid[0].regexes == nil {
		t.Fatal("regexes should be compiled and stored")
	}
	r, ok := valid[0].regexes["regex_match:content"]
	if !ok || r == nil {
		t.Fatal("regex should be stored under key 'regex_match:content'")
	}
	if !r.MatchString("Run BASH now") {
		t.Error("compiled regex should be case-insensitive (prefix (?i))")
	}
}

// Dotall 模式前缀注入
func TestValidateDotallRegex(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "high", AssetType: "skill", Dotall: true,
		Match: MatchNode{raw: map[string]any{"field": "content", "op": "regex_match", "value": "a.b"}}}}
	valid, errs := Validate(rules)
	if len(errs) != 0 || len(valid) != 1 {
		t.Fatalf("want 1 valid 0 err, got %d %v", len(valid), errs)
	}
	r := valid[0].regexes["regex_match:content"]
	// (?s) 使 . 匹配换行
	if !r.MatchString("a\nb") {
		t.Error("dotall regex should match newline with .")
	}
}

// post_exclude 正则编译失败也报错
func TestValidatePostExcludeBadRegexFails(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "high", AssetType: "skill",
		Match:      MatchNode{raw: map[string]any{"field": "c", "op": "exists"}},
		PostExclude: []string{"(?P<bad"}}}
	_, errs := Validate(rules)
	if len(errs) == 0 {
		t.Fatal("bad post_exclude regex should fail")
	}
}

// 多条规则:部分合法部分非法
func TestValidateMixedRules(t *testing.T) {
	rules := []Rule{
		{ID: "good", Severity: "high", AssetType: "settings",
			Match: MatchNode{raw: map[string]any{"field": "x", "op": "exists"}}},
		{ID: "bad", Severity: "nope", AssetType: "settings"},
	}
	valid, errs := Validate(rules)
	if len(valid) != 1 || len(errs) != 1 {
		t.Fatalf("want 1 valid 1 err, got valid=%d errs=%d", len(valid), len(errs))
	}
	if valid[0].ID != "good" {
		t.Errorf("valid rule should be 'good', got %s", valid[0].ID)
	}
	if errs[0].RuleID != "bad" {
		t.Errorf("error rule should be 'bad', got %s", errs[0].RuleID)
	}
}

// RuleLoadError 带 Source
func TestValidateErrorCarriesSource(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "high", AssetType: "bogus", Source: "/path/rules.yaml"}}
	_, errs := Validate(rules)
	if len(errs) != 1 {
		t.Fatalf("want 1 err, got %d", len(errs))
	}
	if errs[0].Source != "/path/rules.yaml" {
		t.Errorf("Source should be /path/rules.yaml, got %q", errs[0].Source)
	}
}

// not 内多布尔键(map 有 and+or)报错
func TestValidateNotMultipleBoolKeysFails(t *testing.T) {
	rules := []Rule{{ID: "x", Severity: "high", AssetType: "settings",
		Match: MatchNode{raw: map[string]any{"not": map[string]any{
			"and": []any{map[string]any{"field": "a", "op": "exists"}},
			"or":  []any{map[string]any{"field": "b", "op": "exists"}},
		}}}}}
	_, errs := Validate(rules)
	if len(errs) == 0 {
		t.Fatal("not child with multiple boolean keys should fail")
	}
}
