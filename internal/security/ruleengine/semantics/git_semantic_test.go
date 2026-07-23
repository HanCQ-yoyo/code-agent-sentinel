package semantics

import "testing"

// TestGitSemantic_BranchForceDelete: git branch -D 是破坏性,语义应识别为 Deny。
func TestGitSemantic_BranchForceDelete(t *testing.T) {
	r := GitSemanticDecision("git branch -D feature")
	if r.Decision != Deny {
		t.Errorf("branch -D: got %v want Deny (rule=%s)", r.Decision, r.RuleID)
	}
	if r.RuleID != "git.branch-force-delete" {
		t.Errorf("branch -D rule: got %q want git.branch-force-delete", r.RuleID)
	}
}

// TestGitSemantic_CommitDataAreaNotDestructive: git commit -m "rm -rf /"
// —— rm -rf 在 -m 数据区(参数),不是真执行,语义应返回 Safe。
func TestGitSemantic_CommitDataAreaNotDestructive(t *testing.T) {
	r := GitSemanticDecision(`git commit -m "rm -rf /"`)
	if r.Decision != Safe {
		t.Errorf("commit -m data area: got %v want Safe (数据区不报)", r.Decision)
	}
}

// TestGitSemantic_ResetHard: git reset --hard 是破坏性,语义应 Deny。
func TestGitSemantic_ResetHard(t *testing.T) {
	r := GitSemanticDecision("git reset --hard origin/main")
	if r.Decision != Deny {
		t.Errorf("reset --hard: got %v want Deny (rule=%s)", r.Decision, r.RuleID)
	}
	if r.RuleID != "git.reset-hard" {
		t.Errorf("reset --hard rule: got %q want git.reset-hard", r.RuleID)
	}
}

// TestGitSemantic_UnknownToRegex: 普通命令(git log)语义无法判定 → Unknown,交回正则层。
func TestGitSemantic_UnknownToRegex(t *testing.T) {
	r := GitSemanticDecision("git log --oneline")
	if r.Decision != Unknown {
		t.Errorf("git log: got %v want Unknown", r.Decision)
	}
}

// TestGitSemantic_CheckoutNewBranchSafe: git checkout -b 是安全(新建分支)。
func TestGitSemantic_CheckoutNewBranchSafe(t *testing.T) {
	r := GitSemanticDecision("git checkout -b feature")
	if r.Decision != Safe {
		t.Errorf("checkout -b: got %v want Safe", r.Decision)
	}
}

// 额外 Deny 路径覆盖(Task 8 brief 鼓励补 edge case)。

func TestGitSemantic_StashDrop(t *testing.T) {
	r := GitSemanticDecision("git stash drop stash@{0}")
	if r.Decision != Deny {
		t.Errorf("stash drop: got %v want Deny (rule=%s)", r.Decision, r.RuleID)
	}
	if r.RuleID != "git.stash-drop" {
		t.Errorf("stash drop rule: got %q want git.stash-drop", r.RuleID)
	}
}

func TestGitSemantic_StashClear(t *testing.T) {
	r := GitSemanticDecision("git stash clear")
	if r.Decision != Deny {
		t.Errorf("stash clear: got %v want Deny", r.Decision)
	}
}

func TestGitSemantic_CleanForce(t *testing.T) {
	r := GitSemanticDecision("git clean -fd")
	if r.Decision != Deny {
		t.Errorf("clean -fd: got %v want Deny (rule=%s)", r.Decision, r.RuleID)
	}
	if r.RuleID != "git.clean-force" {
		t.Errorf("clean -f rule: got %q want git.clean-force", r.RuleID)
	}
}

func TestGitSemantic_PushForce(t *testing.T) {
	r := GitSemanticDecision("git push -f origin feature")
	if r.Decision != Deny {
		t.Errorf("push -f: got %v want Deny (rule=%s)", r.Decision, r.RuleID)
	}
	if r.RuleID != "git.push-force-short" {
		t.Errorf("push -f rule: got %q want git.push-force-short", r.RuleID)
	}
}

func TestGitSemantic_CheckoutDiscard(t *testing.T) {
	// git checkout -- <path> 丢弃工作区改动,破坏性。
	r := GitSemanticDecision("git checkout -- file.txt")
	if r.Decision != Deny {
		t.Errorf("checkout --: got %v want Deny (rule=%s)", r.Decision, r.RuleID)
	}
	if r.RuleID != "git.checkout-discard" {
		t.Errorf("checkout -- rule: got %q want git.checkout-discard", r.RuleID)
	}
}

func TestGitSemantic_TagMessageSafe(t *testing.T) {
	// git tag -m "rm -rf /" 同 commit -m,数据区字面量不执行。
	r := GitSemanticDecision(`git tag -a -m "rm -rf /" v1.0`)
	if r.Decision != Safe {
		t.Errorf("tag -m data area: got %v want Safe", r.Decision)
	}
}

func TestGitSemantic_RestoreStagedSafe(t *testing.T) {
	// git restore --staged 仅影响索引,安全。
	r := GitSemanticDecision("git restore --staged file.txt")
	if r.Decision != Safe {
		t.Errorf("restore --staged: got %v want Safe", r.Decision)
	}
}

func TestGitSemantic_NonGitCommand(t *testing.T) {
	// 非 git 命令 → Unknown(交给正则层)。
	r := GitSemanticDecision("rm -rf /")
	if r.Decision != Unknown {
		t.Errorf("non-git: got %v want Unknown", r.Decision)
	}
}

func TestGitSemantic_SudoGitPrefix(t *testing.T) {
	// sudo git reset --hard 仍应识别。
	r := GitSemanticDecision("sudo git reset --hard")
	if r.Decision != Deny {
		t.Errorf("sudo git reset --hard: got %v want Deny", r.Decision)
	}
}

func TestGitSemantic_GitDirGlobalFlag(t *testing.T) {
	// git -C /path reset --hard 应跳过 -C /path,识别 reset --hard。
	r := GitSemanticDecision("git -C /repo reset --hard")
	if r.Decision != Deny {
		t.Errorf("git -C ... reset --hard: got %v want Deny", r.Decision)
	}
}

// TestGitSemantic_ConfigOverrideFlag: git -c user.name=x reset --hard
// 应跳过 -c user.name=x,识别 reset --hard。
// 回归 review Important #1:此前 -c 未被 stripGitGlobalFlags 剥离,sub 变成 "-c"
// → Unknown(漏报破坏性 reset)。
func TestGitSemantic_ConfigOverrideFlag(t *testing.T) {
	r := GitSemanticDecision("git -c user.name=x reset --hard")
	if r.Decision != Deny {
		t.Errorf("git -c ... reset --hard: got %v want Deny (rule=%s)", r.Decision, r.RuleID)
	}
	if r.RuleID != "git.reset-hard" {
		t.Errorf("git -c ... reset --hard rule: got %q want git.reset-hard", r.RuleID)
	}
}

// TestGitSemantic_GitDirEqualsForm: git --git-dir=/repo reset --hard
// 应跳过 --git-dir=/repo(= 形式),识别 reset --hard。
// 回归 review Minor #1:--git-dir=/path 等 = 形式未剥离。
func TestGitSemantic_GitDirEqualsForm(t *testing.T) {
	r := GitSemanticDecision("git --git-dir=/repo reset --hard")
	if r.Decision != Deny {
		t.Errorf("git --git-dir=... reset --hard: got %v want Deny (rule=%s)", r.Decision, r.RuleID)
	}
	if r.RuleID != "git.reset-hard" {
		t.Errorf("git --git-dir=... reset --hard rule: got %q want git.reset-hard", r.RuleID)
	}
}
