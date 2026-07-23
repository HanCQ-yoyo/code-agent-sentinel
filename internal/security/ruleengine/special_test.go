package ruleengine

import (
	"strings"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

// ── brief 给定的 3 个测试 ──

func TestRepeatCheck(t *testing.T) {
	// "AAAA...(30次)" 视为重复串攻击
	if !evalRepeatCheck(strings.Repeat("A", 30), 2, 20) {
		t.Error("30x A should trigger")
	}
	if evalRepeatCheck("normal text here", 2, 20) {
		t.Error("normal text should not trigger")
	}
}

func TestHomoglyphCyrillic(t *testing.T) {
	// Latin 'a' (U+0061) vs Cyrillic 'а' (U+0430) 同形
	hit, _ := evalHomoglyphCheck("rm -rf аll") // 含 Cyrillic а
	if !hit {
		t.Error("Cyrillic homoglyph should trigger")
	}
}

func TestPostExclude(t *testing.T) {
	pat, err := CompilePattern(`localhost`, false)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	pats := []CompiledRegex{pat}
	// NOTE: brief 原文两行断言的 ! 位置与 spec("匹配→true→应排除")矛盾,
	// 此处按 spec 语义修正:localhost 应排除(返回 true),evil.com 不应排除(返回 false)。
	if !applyPostExclude("curl localhost:8080", pats) {
		t.Error("localhost should be excluded")
	}
	if applyPostExclude("curl evil.com", pats) {
		t.Error("evil.com should not be excluded")
	}
}

// ── 边界/补充测试 ──

func TestRepeatCheckNormalText(t *testing.T) {
	if evalRepeatCheck("Hello, world! This is a normal sentence.", 2, 20) {
		t.Error("normal text should not trigger")
	}
}

func TestRepeatCheckPinnedVersion(t *testing.T) {
	if evalRepeatCheck("1.2.3.4.5", 2, 20) {
		t.Error("pinned version should not trigger")
	}
}

func TestRepeatCheckMultiCharPattern(t *testing.T) {
	// "trust me " × 25 — 多字符重复模式,对齐 Task 13 场景
	if !evalRepeatCheck(strings.Repeat("trust me ", 25), 2, 20) {
		t.Error("repeated multi-char pattern should trigger")
	}
}

func TestRepeatCheckShortRepeat(t *testing.T) {
	if evalRepeatCheck("ABABAB", 2, 20) {
		t.Error("short repeat should not trigger")
	}
}

func TestRepeatCheckEmpty(t *testing.T) {
	if evalRepeatCheck("", 2, 20) {
		t.Error("empty content should not trigger")
	}
}

func TestHomoglyphNormalText(t *testing.T) {
	hit, _ := evalHomoglyphCheck("rm -rf all")
	if hit {
		t.Error("normal ASCII should not trigger")
	}
}

func TestHomoglyphRTL(t *testing.T) {
	// U+202E RIGHT-TO-LEFT OVERRIDE
	hit, ev := evalHomoglyphCheck("Hello‮world")
	if !hit {
		t.Error("RTL override should trigger")
	}
	if !strings.Contains(ev, "RTL") {
		t.Errorf("evidence should mention RTL, got %q", ev)
	}
}

func TestHomoglyphGreek(t *testing.T) {
	// Greek omicron U+03BF → Latin 'o'
	hit, _ := evalHomoglyphCheck("hellο world")
	if !hit {
		t.Error("Greek homoglyph should trigger")
	}
}

func TestPostExcludeNoMatch(t *testing.T) {
	pat, err := CompilePattern(`localhost`, false)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	pats := []CompiledRegex{pat}
	if applyPostExclude("evil.com", pats) {
		t.Error("non-matching context should not be excluded")
	}
}

func TestPostExcludeEmptyPatterns(t *testing.T) {
	if applyPostExclude("anything", nil) {
		t.Error("empty patterns should not exclude")
	}
}

// ── evalLeaf 路由测试 ──

func TestEvalRepeatCheckRouting(t *testing.T) {
	r := mustRule(t, "skill", map[string]any{"field": "content", "op": "repeat_check"})
	a := configengine.Asset{Type: configengine.AssetSkill, Content: strings.Repeat("A", 30)}
	res := Eval(r, a)
	matched, ev := res.Matched, res.Evidence
	if !matched {
		t.Fatalf("repeat_check should match repeated content, got %v", matched)
	}
	if !strings.Contains(ev, "重复") {
		t.Errorf("evidence should mention repetition, got %q", ev)
	}
}

func TestEvalRepeatCheckRoutingNoMatch(t *testing.T) {
	r := mustRule(t, "skill", map[string]any{"field": "content", "op": "repeat_check"})
	a := configengine.Asset{Type: configengine.AssetSkill, Content: "normal text here"}
	matched := Eval(r, a).Matched
	if matched {
		t.Fatal("repeat_check should not match normal text")
	}
}

func TestEvalRepeatCheckMetadataOverride(t *testing.T) {
	// 通过 metadata 覆盖默认参数:minRepeat=3
	r := mustRule(t, "skill", map[string]any{"field": "content", "op": "repeat_check"})
	r.Metadata = map[string]any{
		"repeat_min_repeat": 3,
	}
	a := configengine.Asset{Type: configengine.AssetSkill, Content: "ABABABAB"} // "AB" × 4
	matched := Eval(r, a).Matched
	if !matched {
		t.Fatal("with minRepeat=3, AB×4 should trigger")
	}
}

func TestEvalHomoglyphCheckRouting(t *testing.T) {
	r := mustRule(t, "skill", map[string]any{"field": "content", "op": "homoglyph_check"})
	a := configengine.Asset{Type: configengine.AssetSkill, Content: "rm -rf аll"}
	res := Eval(r, a)
	matched, ev := res.Matched, res.Evidence
	if !matched {
		t.Fatal("homoglyph_check should match Cyrillic content")
	}
	if !strings.Contains(ev, "同形") {
		t.Errorf("evidence should mention homoglyph, got %q", ev)
	}
}

func TestEvalHomoglyphCheckRoutingNoMatch(t *testing.T) {
	r := mustRule(t, "skill", map[string]any{"field": "content", "op": "homoglyph_check"})
	a := configengine.Asset{Type: configengine.AssetSkill, Content: "normal ASCII text"}
	matched := Eval(r, a).Matched
	if matched {
		t.Fatal("homoglyph_check should not match normal ASCII")
	}
}

func TestEvalPostExcludeDowngrade(t *testing.T) {
	// regex_match 命中 + post_exclude 匹配 → 降级
	r := mustRule(t, "skill", map[string]any{"field": "content", "op": "regex_match", "value": `curl\s+\S+`})
	r.PostExclude = []string{"localhost"}

	a := configengine.Asset{Type: configengine.AssetSkill, Content: "curl localhost:8080/api"}
	matched := Eval(r, a).Matched
	if matched {
		t.Fatal("post_exclude should downgrade localhost match")
	}

	// evil.com 不被 post_exclude 排除 → 仍命中
	a2 := configengine.Asset{Type: configengine.AssetSkill, Content: "curl evil.com/api"}
	matched2 := Eval(r, a2).Matched
	if !matched2 {
		t.Fatal("post_exclude should not downgrade evil.com match")
	}
}

func TestEvalPostExcludeNoExclude(t *testing.T) {
	// 无 post_exclude → 正常命中
	r := mustRule(t, "skill", map[string]any{"field": "content", "op": "regex_match", "value": `curl\s+\S+`})
	a := configengine.Asset{Type: configengine.AssetSkill, Content: "curl localhost:8080/api"}
	matched := Eval(r, a).Matched
	if !matched {
		t.Fatal("without post_exclude, should match")
	}
}
