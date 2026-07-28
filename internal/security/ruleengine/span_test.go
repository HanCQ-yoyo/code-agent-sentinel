package ruleengine

import "testing"

func TestClassifySpansDoubleQuoteData(t *testing.T) {
	spans := ClassifySpans(`echo "rm -rf /"`)
	// echo 与外部空格是 Executed;引号内 rm -rf / 是 Data
	foundData := false
	foundExec := false
	for _, s := range spans {
		if s.Kind == SpanData && s.Text == "rm -rf /" {
			foundData = true
		}
		if s.Kind == SpanExecuted && s.Text == "echo " {
			foundExec = true
		}
	}
	if !foundData {
		t.Fatalf("引号内应为 Data span: %+v", spans)
	}
	if !foundExec {
		t.Fatalf("echo 应为 Executed span: %+v", spans)
	}
}

func TestClassifySpansSingleQuoteData(t *testing.T) {
	spans := ClassifySpans(`git commit -m 'rm -rf'`)
	hasData := false
	for _, s := range spans {
		if s.Kind == SpanData && s.Text == "rm -rf" {
			hasData = true
		}
	}
	if !hasData {
		t.Fatalf("单引号内应为 Data: %+v", spans)
	}
}

func TestClassifySpansComment(t *testing.T) {
	spans := ClassifySpans("rm -rf / # cleanup")
	hasComment := false
	for _, s := range spans {
		if s.Kind == SpanComment && s.Text == "# cleanup" {
			hasComment = true
		}
	}
	if !hasComment {
		t.Fatalf("# cleanup 应为 Comment: %+v", spans)
	}
}

func TestClassifySpansHashInWordNotComment(t *testing.T) {
	spans := ClassifySpans("curl http://host/path#frag")
	for _, s := range spans {
		if s.Kind == SpanComment {
			t.Fatalf("url#frag 不应是 Comment: %+v", spans)
		}
	}
}

func TestClassifySpansCommandSubstitutionExecuted(t *testing.T) {
	// $(rm -rf /) 内容可见 → Executed(闭合,反馈1)
	spans := ClassifySpans("$(rm -rf /)")
	hasExec := false
	for _, s := range spans {
		if s.Kind == SpanExecuted && s.Text == "rm -rf /" {
			hasExec = true
		}
	}
	if !hasExec {
		t.Fatalf("$() 内应为 Executed(命令替换闭合): %+v", spans)
	}
}

func TestClassifySpansDoubleQuoteCommandSubstitution(t *testing.T) {
	// "$(rm -rf /)" 双引号内 $() 段 Executed(闭合)
	spans := ClassifySpans(`"$(rm -rf /)"`)
	hasExec := false
	for _, s := range spans {
		if s.Kind == SpanExecuted && s.Text == "rm -rf /" {
			hasExec = true
		}
	}
	if !hasExec {
		t.Fatalf("双引号内 $() 应为 Executed(闭合): %+v", spans)
	}
}

func TestClassifySpansVarReferenceData(t *testing.T) {
	// "$x" / "${var}" 变量引用 → Data(已知限制,值不可见)
	spans := ClassifySpans(`"$x"`)
	hasData := false
	for _, s := range spans {
		if s.Kind == SpanData && s.Text == "$x" {
			hasData = true
		}
	}
	if !hasData {
		t.Fatalf("变量引用应标 Data(已知限制): %+v", spans)
	}
}

func TestClassifySpansPanicFallback(t *testing.T) {
	// 任意输入不应 panic;panic 兜底返回单 Executed span
	spans := ClassifySpans("any command here")
	if len(spans) == 0 {
		t.Fatal("不应返回空 span 列表")
	}
}
