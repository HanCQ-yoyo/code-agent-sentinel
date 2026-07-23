package ruleengine

import (
	"testing"

	"code-agent-sentinel/internal/configengine"
)

// 验证 destructive_commands.yaml 能被 LoadBuiltin 加载,且样例规则可求值。
// Task 3 的骨架测试:文件存在 + 样例规则 destructive.sample.should-exist 注册。
// Task 4 起追加真实 git 域规则后,样例规则被删除,本测试改为验证 git 域规则加载。
func TestDestructiveRules_Load(t *testing.T) {
	rules, errs := LoadBuiltin()
	if len(errs) > 0 {
		t.Fatalf("load errors: %v", errs)
	}
	// git 域应至少有 12 条规则
	gitCount := 0
	for _, r := range rules {
		if d, ok := r.Metadata["domain"].(string); ok && d == "git" {
			gitCount++
		}
	}
	if gitCount < 12 {
		t.Errorf("expected ≥12 destructive.git.* rules, got %d", gitCount)
	}
}

// filterRulesByDomain 按 metadata.domain 过滤规则。
func filterRulesByDomain(rules []Rule, domain string) []Rule {
	var out []Rule
	for _, r := range rules {
		if d, ok := r.Metadata["domain"].(string); ok && d == domain {
			out = append(out, r)
		}
	}
	return out
}

// makeAssetWithField 合成 Asset:field=content 写 Content,其余写 Fields[field]。
func makeAssetWithField(field, cmd string) configengine.Asset {
	a := configengine.Asset{Type: configengine.AssetHook}
	if field == "content" {
		a.Content = cmd
	} else {
		a.Fields = map[string]any{field: cmd}
	}
	return a
}

// evalRuleMatch 调 ruleengine.Eval 返回是否命中。
func evalRuleMatch(r Rule, a configengine.Asset) (bool, string) {
	res := Eval(r, a)
	return res.Matched, res.Evidence
}

// evalRulesForCommand 合成 Asset 对 cmd 跑规则,返回首个命中规则 id。
func evalRulesForCommand(t *testing.T, rules []Rule, field, cmd string) string {
	t.Helper()
	asset := makeAssetWithField(field, cmd)
	for _, r := range rules {
		matched, _ := evalRuleMatch(r, asset)
		if matched {
			return r.ID
		}
	}
	return ""
}

// TestDestructive_GitDomain — Task 4:git 域 12 条 dest 规则 + safe post_exclude。
// 覆盖:5 条 dest 命中 + 3 条 safe 不误报(含 git commit -m "rm -rf /" 数据区隔)。
func TestDestructive_GitDomain(t *testing.T) {
	rules, errs := LoadBuiltin()
	if len(errs) > 0 {
		t.Fatalf("LoadBuiltin errors: %v", errs)
	}
	gitRules := filterRulesByDomain(rules, "git")
	if len(gitRules) < 12 {
		t.Fatalf("expected ≥12 git rules, got %d", len(gitRules))
	}

	cases := []struct {
		name   string
		cmd    string
		field  string // command / content / allow
		wantID string // 期望命中的规则 id(空=不应命中)
	}{
		// dest 命中
		{"reset-hard", "git reset --hard origin/main", "command", "destructive.git.reset-hard"},
		{"checkout-discard", "git checkout -- file.txt", "command", "destructive.git.checkout-discard"},
		{"clean-force", "git clean -fd", "command", "destructive.git.clean-force"},
		{"branch-force-delete", "git branch -D feature", "command", "destructive.git.branch-force-delete"},
		{"push-force-short", "git push -f origin main", "command", "destructive.git.push-force-short"},
		{"push-force-long", "git push --force origin main", "command", "destructive.git.push-force-long"},
		{"stash-drop", "git stash drop", "command", "destructive.git.stash-drop"},
		{"stash-clear", "git stash clear", "command", "destructive.git.stash-clear"},
		// safe 不误报(对应 safe_pattern 应被 post_exclude 排除)
		{"checkout-new-branch-safe", "git checkout -b feature", "command", ""},
		{"checkout-orphan-safe", "git checkout --orphan newbranch", "command", ""},
		{"clean-dry-run-safe", "git clean -nfd", "command", ""},
		{"git-commit-msg-safe", "git commit -m \"rm -rf /\"", "command", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hitID := evalRulesForCommand(t, gitRules, c.field, c.cmd)
			if hitID != c.wantID {
				t.Errorf("cmd=%q field=%s: got %q want %q", c.cmd, c.field, hitID, c.wantID)
			}
		})
	}
}
