package ruleengine

import (
	"reflect"
	"testing"
)

func TestSplitCommandAndAnd(t *testing.T) {
	got := SplitCommand("a && b")
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSplitCommandSemicolon(t *testing.T) {
	got := SplitCommand("a; b")
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSplitCommandOrOr(t *testing.T) {
	got := SplitCommand("a || b")
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSplitCommandPipe(t *testing.T) {
	got := SplitCommand("a | b")
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSplitCommandQuoteNotSplit(t *testing.T) {
	got := SplitCommand(`echo "a && b"`)
	want := []string{`echo "a && b"`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("引号内不应拆: got %v want %v", got, want)
	}
}

func TestSplitCommandSubstNotSplit(t *testing.T) {
	got := SplitCommand("$(a && b)")
	want := []string{"$(a && b)"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("$() 内不应拆: got %v want %v", got, want)
	}
}

func TestSplitCommandParenNotSplit(t *testing.T) {
	got := SplitCommand("(a && b)")
	want := []string{"(a && b)"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("() 内不应拆: got %v want %v", got, want)
	}
}

func TestSplitCommandSingle(t *testing.T) {
	got := SplitCommand("rm -rf /")
	want := []string{"rm -rf /"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("单片段: got %v want %v", got, want)
	}
}

func TestSplitCommandMultiChain(t *testing.T) {
	got := SplitCommand("a && b; c || d | e")
	want := []string{"a", "b", "c", "d", "e"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("多分隔: got %v want %v", got, want)
	}
}

func TestSplitCommandPanicFallback(t *testing.T) {
	// 注入 panic,验证兜底返回整条单片段(fail-open 铁律:不因拆分失败漏报)。
	// 关键:SplitCommand 用命名返回值 segs + deferred recover 赋值,
	// 否则 Go 的 defer recover 无法修改非命名返回值(Task 1 review 曾捕获此 bug)。
	orig := splitImplFn
	splitImplFn = func(s string) []string { panic("injected") }
	defer func() { splitImplFn = orig }()
	got := SplitCommand("rm -rf /")
	want := []string{"rm -rf /"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("panic 兜底应返回整条单片段: got %v want %v", got, want)
	}
}
