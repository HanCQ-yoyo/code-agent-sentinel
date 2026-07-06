package configengine

import "testing"

func TestParseHooksFromData(t *testing.T) {
	// 模拟一个 hooks/hooks.json 内容(插件 hook 布局):顶层是 settings 的 hooks map。
	raw := `{
  "PreToolUse": [
    {"matcher": "Bash", "hooks": [{"type": "command", "command": "echo hi"}]}
  ]
}`
	// parseHooksFromData 接受的是 settings.json 整体或 hooks.json 整体;
	// hooks.json 顶层就是 hooks map,event 名为 key。
	got := parseHooksFromData([]byte(raw), "/tmp/h.json", ScopePlugin)
	if len(got) != 1 {
		t.Fatalf("got %d hooks, want 1", len(got))
	}
	h := got[0]
	if h.Type != AssetHook || h.Scope != ScopePlugin {
		t.Fatalf("hook = %+v, want type=hook scope=plugin", h)
	}
	if h.Fields["command"] != "echo hi" {
		t.Fatalf("command = %v, want echo hi", h.Fields["command"])
	}
}

func TestParseHooksFromDataMalformed(t *testing.T) {
	// 损坏 JSON 返回 nil(不 panic),不产出占位 —— 调用方决定如何处理。
	got := parseHooksFromData([]byte("{not json"), "/tmp/h.json", ScopePlugin)
	if got != nil {
		t.Fatalf("malformed = %+v, want nil", got)
	}
}
