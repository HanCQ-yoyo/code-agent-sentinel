package ruleengine

// Confidence 是命中的置信度。调用方按 GuardConfig.Mode 解释 Unknown。
type Confidence int

const (
	ConfHigh    Confidence = iota // 命中在 executed span 中心(词中部),高置信
	ConfLow                       // 命中在 span 边界/引号紧邻/转义符后/Data 区,低置信
	ConfUnknown                   // 打分自身异常(panic),由调用方按 Mode 解释
)

// ScoreConfidence 根据命中在 span 中的位置打分。
//   - 命中落在 SpanData/SpanComment → ConfLow(防御,不该发生)
//   - 命中落在 SpanExecuted 且距 span 边界 > 2 字节 → ConfHigh
//   - 命中紧贴 span 边界(距边界 ≤2 字节)→ ConfLow
//   - 无 span 信息或未找到包含命中的 span → ConfLow(保守)
//   - panic → ConfUnknown(strict→当 High→deny,lenient→当 Low→ask)
//
// panic 兜底:用命名返回值 c + deferred recover 赋值,确保 panic 时返回值被真正改写
// (Go 的 defer recover 无法修改非命名返回值——Task 1 review 曾捕获此 bug)。
// 这替代了 brief 骨架中的包级 panicConfidence 标记方案(该方案有竞态且对调用方不可见)。
// 调用方(guard_cmd.evaluateSegment)直接读返回的 Confidence;若为 ConfUnknown,
// 按 Mode 调用 .ForMode(mode) 解释为 High/low。
func ScoreConfidence(matchText string, matchOffset int, spans []Span) (c Confidence) {
	defer func() {
		if r := recover(); r != nil {
			// panic 兜底:置信度未知,交调用方按 Mode 解释(strict→High→deny,lenient→Low→ask)。
			// 安全不变量:绝不静默 ConfHigh(over-deny)或 ConfLow(under-protect)。
			c = ConfUnknown
		}
	}()
	c = scoreConfidenceImplFn(matchText, matchOffset, spans)
	return c
}

// scoreConfidenceImplFn 是实现钩子(包级变量,测试可注入 panic),与 span.go
// classifySpansImplFn / split.go splitImplFn 同模式。
var scoreConfidenceImplFn = scoreConfidenceImpl

// scoreConfidenceImpl 是 ScoreConfidence 的实现(无 panic 兜底,由 ScoreConfidence 包裹)。
func scoreConfidenceImpl(matchText string, matchOffset int, spans []Span) Confidence {
	if len(spans) == 0 {
		return ConfLow // 无 span 信息,保守 Low
	}
	matchLen := len(matchText)
	matchEnd := matchOffset + matchLen
	for _, s := range spans {
		if matchOffset < s.Start || matchEnd > s.End {
			continue
		}
		// 命中落在该 span 内
		if s.Kind != SpanExecuted {
			return ConfLow // Data/Comment 区(防御,不该发生)
		}
		distLeft := matchOffset - s.Start
		distRight := s.End - matchEnd
		if distLeft <= 2 || distRight <= 2 {
			return ConfLow // 紧贴边界(引号闭合处/转义符后)
		}
		return ConfHigh
	}
	return ConfLow // 未找到包含命中的 span(保守)
}

// ForMode 把 ConfUnknown 按 Mode 解释为 High(strict)或 Low(lenient),非 Unknown 原样返回。
// 供 guard_cmd.evaluateSegment 用:strict 模式偏保守(当 High→deny),lenient 偏可用(当 Low→ask)。
func (c Confidence) ForMode(mode string) Confidence {
	if c == ConfUnknown {
		if mode == "lenient" {
			return ConfLow
		}
		return ConfHigh // strict 默认(空串/未知 mode 均走严格)
	}
	return c
}

// String 返回 confidence 的字符串表示(记录/UI 用)。
func (c Confidence) String() string {
	switch c {
	case ConfHigh:
		return "high"
	case ConfLow:
		return "low"
	case ConfUnknown:
		return "unknown"
	}
	return "unknown"
}
