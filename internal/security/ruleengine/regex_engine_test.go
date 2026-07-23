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

// TestCompilePattern_FindAllStringIndex_MultiByte 验证 regexp2 路径在多字节 UTF-8
// 输入下返回的是字节偏移(而非 rune 偏移)。lookahead (?!-b\b) 强制走 regexp2Wrapper;
// 前导中文字符(每字 3 字节)使 rune 偏移与字节偏移不一致 —— 若 runeOffsetToByte
// 实现错误,text[start:end] 会切到错误位置(如切到中文字符中间)或越界。
//
// 覆盖 review Important #1(字节偏移正确性)+ Minor #2(locationsFromOffsets 端到端)。
func TestCompilePattern_FindAllStringIndex_MultiByte(t *testing.T) {
	// 字节布局:中(0-2) 文(3-5) 空格(6) g(7) i(8) t(9) 空格(10) c(11)...n(23) 空格(24) 文(25-27) 本(28-30)
	text := "中文 git checkout main 文本"
	// lookahead (?!-b\b) 强制走 regexp2Wrapper(非 RE2)
	re, err := CompilePattern(`git\s+checkout\s+(?!-b\b)\S+`, false)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	idxs := re.FindAllStringIndex(text)
	if len(idxs) != 1 {
		t.Fatalf("expected 1 match, got %d", len(idxs))
	}
	start, end := idxs[0][0], idxs[0][1]
	// 期望字节区间 [7, 24):"git checkout main"(前导中文占 6 字节 + 空格 1 字节 = 7)
	if start != 7 || end != 24 {
		t.Errorf("byte offsets = [%d, %d), want [7, 24)", start, end)
	}
	// 关键断言:字节偏移切出的子串必须等于实际匹配文本。
	// 若实现错误返回 rune 偏移(start=3),text[3:20]="文 git checkout " ≠ 匹配文本。
	if got := text[start:end]; got != "git checkout main" {
		t.Errorf("text[%d:%d] = %q, want %q", start, end, got, "git checkout main")
	}

	// Minor #2:端到端 Location 验证(多字节 + lookahead 经 locationsFromOffsets)。
	// 验证字节偏移传入 locationsFromOffsets 后得到正确的 1-based 行列。
	locs := locationsFromOffsets(text, idxs)
	if len(locs) != 1 {
		t.Fatalf("expected 1 location, got %d", len(locs))
	}
	// 单行文本,第 1 行。startCol = byte 7 → 列 8(1-based:7-0+1)。
	// endCol:末字节 23 → 列 24,半开区间再 +1 = 25。
	if locs[0].Line != 1 {
		t.Errorf("Line = %d, want 1", locs[0].Line)
	}
	if locs[0].StartCol != 8 || locs[0].EndCol != 25 {
		t.Errorf("col = [%d, %d), want [8, 25)", locs[0].StartCol, locs[0].EndCol)
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
