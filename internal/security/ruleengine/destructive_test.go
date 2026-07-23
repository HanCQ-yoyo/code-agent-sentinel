package ruleengine

import "testing"

// 验证 destructive_commands.yaml 能被 LoadBuiltin 加载,且样例规则可求值。
// Task 3 的骨架测试:文件存在 + 样例规则 destructive.sample.should-exist 注册。
// Task 4-7 在此文件追加 5 域真实规则后,本测试仍应通过(样例规则不被删除)。
func TestDestructiveRules_Load(t *testing.T) {
	rules, errs := LoadBuiltin()
	if len(errs) > 0 {
		t.Fatalf("load errors: %v", errs)
	}
	// 样例规则 destructive.sample.should-exist 必须存在
	found := false
	for _, r := range rules {
		if r.ID == "destructive.sample.should-exist" {
			found = true
			break
		}
	}
	if !found {
		t.Error("destructive.sample.should-exist not loaded")
	}
}
