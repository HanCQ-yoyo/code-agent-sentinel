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
		// push-force-long/short 的 (?![-a-z]) lookahead 使 --force-with-lease 不匹配
		{"push-force-with-lease-safe", "git push --force-with-lease origin main", "command", ""},
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

// TestDestructive_FilesystemDomain — Task 5:filesystem 域 26 条 dest 规则 + safe→post_exclude。
// 覆盖:rm-rf /、rm -rf ~、find / -delete、unlink /etc/passwd 等 dest 命中;
// safe 不误报:rm -i file(无 -rf 标志不匹配)、rm /tmp/foo(post_exclude 排除 tmp 路径)。
//
// 规则名对齐 dcg core/filesystem.rs 的 pattern name(如 rm-rf-root-home 而非 rm-root-absolute)。
func TestDestructive_FilesystemDomain(t *testing.T) {
	rules, errs := LoadBuiltin()
	if len(errs) > 0 {
		t.Fatalf("LoadBuiltin errors: %v", errs)
	}
	fsRules := filterRulesByDomain(rules, "filesystem")
	if len(fsRules) < 26 {
		t.Fatalf("expected ≥26 filesystem rules, got %d", len(fsRules))
	}

	cases := []struct {
		name   string
		cmd    string
		field  string
		wantID string
	}{
		// dest 命中(Critical:root/home/system 目标)
		{"rm-rf-root", "rm -rf /", "command", "destructive.filesystem.rm-rf-root-home"},
		{"rm-rf-home", "rm -rf ~", "command", "destructive.filesystem.rm-rf-root-home"},
		{"rm-r-f-separate-root", "rm -r -f /", "command", "destructive.filesystem.rm-r-f-separate-root-home"},
		{"rm-recursive-force-root", "rm --recursive --force /", "command", "destructive.filesystem.rm-recursive-force-root-home"},
		{"find-delete-root", "find / -delete", "command", "destructive.filesystem.find-delete-root-home"},
		{"find-delete-etc", "find /etc -delete", "command", "destructive.filesystem.find-delete-root-home"},
		{"unlink-etc", "unlink /etc/passwd", "command", "destructive.filesystem.unlink-root-home"},
		{"truncate-zero-etc", "truncate -s 0 /etc/passwd", "command", "destructive.filesystem.truncate-zero-root-home"},
		{"shred-etc", "shred /etc/passwd", "command", "destructive.filesystem.shred-root-home"},
		{"tar-remove-files-etc", "tar --remove-files -cf out.tar /etc", "command", "destructive.filesystem.tar-remove-files-root-home"},
		{"dd-of-etc", "dd if=/dev/zero of=/etc/passwd", "command", "destructive.filesystem.dd-overwrite-root-home"},
		{"mv-etc", "mv /etc /tmp/x", "command", "destructive.filesystem.mv-sensitive-source-root-home"},
		{"redirect-truncate-etc", "echo data > /etc/passwd", "command", "destructive.filesystem.redirect-truncate-root-home"},
		// dest 命中(High:非 root/home 目标)
		{"rm-rf-general", "rm -rf ./build", "command", "destructive.filesystem.rm-rf-general"},
		{"find-delete-general", "find . -delete", "command", "destructive.filesystem.find-delete-general"},
		{"unlink-general", "unlink ./scratch", "command", "destructive.filesystem.unlink-general"},
		{"truncate-zero-general", "truncate -s 0 ./file", "command", "destructive.filesystem.truncate-zero-general"},
		{"shred-general", "shred ./file", "command", "destructive.filesystem.shred-general"},
		{"tar-remove-files-general", "tar --remove-files -cf out.tar ./src", "command", "destructive.filesystem.tar-remove-files-general"},
		{"dd-overwrite-general", "dd of=./file", "command", "destructive.filesystem.dd-overwrite-general"},
		{"mv-dynamic-path", "mv $VAR /tmp/x", "command", "destructive.filesystem.mv-dynamic-path"},
		// 传播链(Critical:跨段敏感路径传播后强制删除)
		{"cp-sensitive-then-delete", "cp -a /etc /tmp/x && rm -rf /tmp/x", "command", "destructive.filesystem.cp-sensitive-then-delete"},
		{"ln-symlink-sensitive-then-delete", "ln -s /etc /tmp/x && rm -rf /tmp/x", "command", "destructive.filesystem.ln-symlink-sensitive-then-delete"},
		{"rsync-sensitive-then-delete", "rsync -a /etc/ /tmp/dest/ && rm -rf /tmp/dest", "command", "destructive.filesystem.rsync-sensitive-then-delete"},
		// safe 不误报(post_exclude 排除 tmp 路径 / 非破坏形态)
		{"rm-rf-tmp-safe", "rm -rf /tmp/foo", "command", ""},                 // post_exclude: rm-rf-tmp
		{"rm-recursive-force-tmp-safe", "rm --recursive --force /tmp/foo", "command", ""}, // post_exclude: rm-recursive-force-tmp
		{"find-delete-tmp-safe", "find /tmp/foo -delete", "command", ""},     // post_exclude: find-delete-tmp
		{"unlink-tmp-safe", "unlink /tmp/scratch", "command", ""},            // post_exclude: unlink-tmp
		{"rm-interactive-safe", "rm -i file", "command", ""},                 // -i 无 -rf,不匹配
		{"chmod-normal", "chmod 644 file", "command", ""},                    // 无 chmod 规则(filesystem 域)
		{"rm-help-safe", "rm --help", "command", ""},                         // 无 -rf,不匹配
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hitID := evalRulesForCommand(t, fsRules, c.field, c.cmd)
			if hitID != c.wantID {
				t.Errorf("cmd=%q field=%s: got %q want %q", c.cmd, c.field, hitID, c.wantID)
			}
		})
	}
}
