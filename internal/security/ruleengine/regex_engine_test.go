package ruleengine

import "testing"

// 需求:含 lookahead 的模式能编译并匹配;纯正则走 RE2 不变;dotall 透传。
// dcg 的 ~737 条规则大量使用 lookahead/lookbehind(RE2 不支持),走 regexp2;
// 纯正则走 RE2(标准库 regexp),行为与原 compileRegexPattern 一致。

func TestCompilePattern_Lookahead(t *testing.T) {
	// lookahead:RE2 不支持,必须走 regexp2
	re, err := CompilePattern(`git\s+checkout\s+(?!-b\b)`, false)
	if err != nil {
		t.Fatalf("compile lookahead pattern: %v", err)
	}
	// (?!-b\b) 排除 checkout -b;checkout main 应命中
	if !re.MatchString("git checkout main") {
		t.Error("expected match for 'git checkout main' (no -b)")
	}
	if re.MatchString("git checkout -b feature") {
		t.Error("expected NO match for 'git checkout -b feature' (excluded by lookahead)")
	}
}

func TestCompilePattern_PlainRE2(t *testing.T) {
	// 纯正则走 RE2,行为不变
	re, err := CompilePattern(`rm\s+-rf`, false)
	if err != nil {
		t.Fatalf("compile plain pattern: %v", err)
	}
	if !re.MatchString("rm -rf /tmp") {
		t.Error("expected match")
	}
}

func TestCompilePattern_FindAllStringIndex(t *testing.T) {
	// 统一接口:FindAllStringIndex 返回 [][2]int(RE2 与 regexp2 一致)
	re, err := CompilePattern(`git\s+checkout\s+(?!-b\b)\S+`, false)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	idxs := re.FindAllStringIndex("git checkout main && git checkout -b x && git checkout dev")
	// 命中 checkout main 和 checkout dev,-b 被排除 → 2 个
	if len(idxs) != 2 {
		t.Errorf("expected 2 matches, got %d", len(idxs))
	}
}

func TestCompilePattern_Dotall(t *testing.T) {
	// dotall 透传:RE2 用 (?s),regexp2 用 Singleline flag。
	// . 匹配换行 → 跨行命中。
	re, err := CompilePattern(`foo.bar`, true)
	if err != nil {
		t.Fatalf("compile dotall pattern: %v", err)
	}
	if !re.MatchString("foo\nbar") {
		t.Error("dotall=true should match across newline")
	}
	// dotall=false 不匹配换行
	re2, err := CompilePattern(`foo.bar`, false)
	if err != nil {
		t.Fatalf("compile non-dotall pattern: %v", err)
	}
	if re2.MatchString("foo\nbar") {
		t.Error("dotall=false should NOT match across newline")
	}
}

func TestCompilePattern_FindString(t *testing.T) {
	// 统一接口 FindString:返回首个匹配的字符串
	re, err := CompilePattern(`rm\s+-rf`, false)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := re.FindString("do rm -rf /tmp now")
	if got != "rm -rf" {
		t.Errorf("FindString = %q, want %q", got, "rm -rf")
	}
}

func TestNeedsBacktracking(t *testing.T) {
	cases := []struct {
		pat  string
		want bool
	}{
		{`rm\s+-rf`, false},
		{`git\s+checkout\s+(?!-b\b)`, true},  // 负向前瞻
		{`(?=.*--staged)`, true},              // 正向前瞻
		{`(?<=foo)bar`, true},                 // 正向后瞻
		{`(?<!sudo)\s+rm`, true},              // 负向后瞻
		{`[a-z]+\d+`, false},
		{`\(\?P<name>\w+\)`, false}, // 命名捕获,非 lookahead
	}
	for _, c := range cases {
		if got := needsBacktracking(c.pat); got != c.want {
			t.Errorf("needsBacktracking(%q)=%v want %v", c.pat, got, c.want)
		}
	}
}
