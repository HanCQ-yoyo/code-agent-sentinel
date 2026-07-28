package ruleengine

import "testing"

// TestScoreConfidenceHighMidSpan 验证命中落在 Executed span 中部(距边界 >2 字节)→ High。
//
// 注意:brief 原用 "rm -rf /" + offset 0,但 ClassifySpans 对该输入产出单 Executed
// span [0,8),命中 "rm" 在 offset 0 → matchEnd 2,distLeft=0 ≤2 → 实为边界(Low),
// 与 "中部 High" 的测试意图矛盾。改用 "echo rm -rf /"(单 Executed span [0,13)),
// 命中 "rm" 在 offset 5 → matchEnd 7,distLeft=5/distRight=6 均 >2 → 真·中部 High。
func TestScoreConfidenceHighMidSpan(t *testing.T) {
	spans := ClassifySpans("echo rm -rf /")
	got := ScoreConfidence("rm", 5, spans)
	if got != ConfHigh {
		t.Fatalf("命中 span 中部应 High: got %v", got)
	}
}

// TestScoreConfidenceLowNearQuoteBoundary 验证命中落在 Data span(引号内)→ Low(防御)。
//
// ClassifySpans(`echo "rm"`) 产出:
//   - [0,5)  Executed "echo "
//   - [6,8)  Data     "rm"
//
// 命中 "rm" 在 offset 6 → matchEnd 8,落在 Data span [6,8) → Low。
func TestScoreConfidenceLowNearQuoteBoundary(t *testing.T) {
	spans := ClassifySpans(`echo "rm"`)
	got := ScoreConfidence("rm", 6, spans) // 6 是引号内 rm 的偏移
	if got != ConfLow {
		t.Fatalf("命中 Data span 应 Low: got %v", got)
	}
}

// TestScoreConfidenceHighLongSpan 验证长 Executed span 中部命中 → High。
//
// ClassifySpans("git reset --hard") 产出单 Executed span [0,16)。
// 命中 "reset" 在 offset 4 → matchEnd 9,distLeft=4/distRight=7 均 >2 → High。
func TestScoreConfidenceHighLongSpan(t *testing.T) {
	spans := ClassifySpans("git reset --hard")
	got := ScoreConfidence("reset", 4, spans)
	if got != ConfHigh {
		t.Fatalf("命中长 span 中部应 High: got %v", got)
	}
}

// TestScoreConfidenceLowAtSpanBoundary 验证命中紧贴 Executed span 边界 → Low。
//
// 补 brief 缺失的边界用例:"rm -rf /" 单 Executed span [0,8),命中 "rm" 在 offset 0
// → matchEnd 2,distLeft=0 ≤2 → Low(紧贴左边界,引号闭合/转义符后同理)。
func TestScoreConfidenceLowAtSpanBoundary(t *testing.T) {
	spans := ClassifySpans("rm -rf /")
	got := ScoreConfidence("rm", 0, spans)
	if got != ConfLow {
		t.Fatalf("紧贴 span 边界应 Low: got %v", got)
	}
}

// TestScoreConfidenceLowNoSpans 验证无 span 信息 → Low(保守)。
func TestScoreConfidenceLowNoSpans(t *testing.T) {
	got := ScoreConfidence("rm", 0, nil)
	if got != ConfLow {
		t.Fatalf("无 span 应保守 Low: got %v", got)
	}
}

// TestScoreConfidenceLowNoContainingSpan 验证命中未被任何 span 包含 → Low(保守)。
// 构造 span [0,3) Executed "rm ",命中 "rm" 在 offset 5(超出所有 span)。
func TestScoreConfidenceLowNoContainingSpan(t *testing.T) {
	spans := []Span{{Kind: SpanExecuted, Text: "rm ", Start: 0, End: 3}}
	got := ScoreConfidence("rm", 5, spans)
	if got != ConfLow {
		t.Fatalf("未包含命中的 span 应保守 Low: got %v", got)
	}
}

// TestScoreConfidenceLowOnComment 验证命中落在 Comment span → Low(防御)。
func TestScoreConfidenceLowOnComment(t *testing.T) {
	spans := []Span{{Kind: SpanComment, Text: "rm -rf", Start: 0, End: 6}}
	got := ScoreConfidence("rm", 0, spans)
	if got != ConfLow {
		t.Fatalf("命中 Comment span 应 Low: got %v", got)
	}
}

// TestScoreConfidenceUnknownOnPanic 验证打分实现 panic 时返回 ConfUnknown。
//
// 关键不变量:panic 必须返回 ConfUnknown(交调用方按 Mode 解释:strict→High→deny,
// lenient→Low→ask),绝不静默 ConfHigh(over-deny)或 ConfLow(under-protect)。
// 此测试用 impl 钩子注入 panic,若兜底返回 ConfLow 或 ConfHigh 则失败。
func TestScoreConfidenceUnknownOnPanic(t *testing.T) {
	orig := scoreConfidenceImplFn
	scoreConfidenceImplFn = func(matchText string, matchOffset int, spans []Span) Confidence {
		panic("injected")
	}
	defer func() { scoreConfidenceImplFn = orig }()
	got := ScoreConfidence("rm", 0, []Span{{Kind: SpanExecuted, Text: "rm", Start: 0, End: 2}})
	if got != ConfUnknown {
		t.Fatalf("panic 兜底应返回 ConfUnknown: got %v", got)
	}
}

// TestConfidenceForMode 验证 ForMode 把 Unknown 按 Mode 解释,非 Unknown 原样返回。
func TestConfidenceForMode(t *testing.T) {
	cases := []struct {
		in   Confidence
		mode string
		want Confidence
	}{
		{ConfUnknown, "strict", ConfHigh},
		{ConfUnknown, "lenient", ConfLow},
		{ConfUnknown, "other", ConfHigh}, // 非 lenient 默认 strict
		{ConfHigh, "strict", ConfHigh},
		{ConfLow, "strict", ConfLow},
		{ConfHigh, "lenient", ConfHigh},
	}
	for _, c := range cases {
		got := c.in.ForMode(c.mode)
		if got != c.want {
			t.Errorf("Confidence(%v).ForMode(%q) = %v, want %v", c.in, c.mode, got, c.want)
		}
	}
}

// TestConfidenceString 验证 String 返回可读表示。
func TestConfidenceString(t *testing.T) {
	cases := []struct {
		in   Confidence
		want string
	}{
		{ConfHigh, "high"},
		{ConfLow, "low"},
		{ConfUnknown, "unknown"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("Confidence(%v).String() = %q, want %q", c.in, got, c.want)
		}
	}
}
