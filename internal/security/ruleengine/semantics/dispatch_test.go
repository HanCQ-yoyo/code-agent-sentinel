package semantics

import "testing"

// TestDispatch_Git:git 域 → GitSemanticDecision。
// git reset --hard 是破坏性子命令,应 Deny。
func TestDispatch_Git(t *testing.T) {
	r := Dispatch("git", "git reset --hard")
	if r.Decision != Deny {
		t.Errorf("git reset --hard: got %v want Deny", r.Decision)
	}
	if r.RuleID != "git.reset-hard" {
		t.Errorf("RuleID = %q, want git.reset-hard", r.RuleID)
	}
}

// TestDispatch_Filesystem:filesystem 域 → RmSemanticDecision。
// rm -rf / 是破坏性递归强制删根,应 Deny。
func TestDispatch_Filesystem(t *testing.T) {
	r := Dispatch("filesystem", "rm -rf /")
	if r.Decision != Deny {
		t.Errorf("rm -rf /: got %v want Deny", r.Decision)
	}
	if r.RuleID != "filesystem.rm-rf-root-home" {
		t.Errorf("RuleID = %q, want filesystem.rm-rf-root-home", r.RuleID)
	}
}

// TestDispatch_Snowflake:database 域 → snowflakeSemanticDecision(提取 --query SQL,跑 ScanSQL)。
// snow sql --query 'DROP TABLE x' 含破坏性 keyword DROP,应 Deny。
func TestDispatch_Snowflake(t *testing.T) {
	r := Dispatch("database", "snow sql --query 'DROP TABLE x'")
	if r.Decision != Deny {
		t.Errorf("snow DROP TABLE: got %v want Deny (reason=%q)", r.Decision, r.Reason)
	}
}

// TestDispatch_UnknownDomain:containers/package_managers 无语义解析器,应返回 Unknown。
// 语义层不处理,交回正则。
func TestDispatch_UnknownDomain(t *testing.T) {
	r := Dispatch("containers", "docker rm -f x")
	if r.Decision != Unknown {
		t.Errorf("containers 无语义解析器: got %v want Unknown", r.Decision)
	}
	// package_managers 同样无解析器
	r2 := Dispatch("package_managers", "npm unpublish x --force")
	if r2.Decision != Unknown {
		t.Errorf("package_managers 无语义解析器: got %v want Unknown", r2.Decision)
	}
}

// TestDispatch_DatabaseNotSnowSQL:database 域但非 snow sql 命令 → Unknown。
// (snowflakeSemanticDecision 只处理 snow sql --query 形式;其他 CLI 命令交回正则。)
func TestDispatch_DatabaseNotSnowSQL(t *testing.T) {
	r := Dispatch("database", "mysql -e 'DROP TABLE x'")
	if r.Decision != Unknown {
		t.Errorf("非 snow sql 命令应交回正则: got %v want Unknown", r.Decision)
	}
}

// TestDispatch_GitSafe:git commit -m 数据区命令 → Safe。
// 验证 Dispatch 把 git 域正确路由到 GitSemanticDecision,Safe 路径正常返回。
func TestDispatch_GitSafe(t *testing.T) {
	r := Dispatch("git", `git commit -m "rm -rf /"`)
	if r.Decision != Safe {
		t.Errorf("git commit -m 数据区: got %v want Safe", r.Decision)
	}
}

// TestDispatchCommand_PriorityGitOverFilesystem 验证 DispatchCommand 优先级分发:
// `git commit -m "rm -rf /"` 中,git 解析器判 Safe(commit -m 数据区),
// filesystem 解析器会误判 Deny(看到 rm -rf / 字面量)。优先级分发应返回 git 的 Safe,
// 避免跨域误判。这是 RulesDetector 实际调用的函数,解决跨域冲突。
func TestDispatchCommand_PriorityGitOverFilesystem(t *testing.T) {
	r := DispatchCommand(`git commit -m "rm -rf /"`)
	if r.Decision != Safe {
		t.Errorf("git commit -m 数据区: DispatchCommand 应返回 git Safe, got %v (ruleID=%s reason=%q)",
			r.Decision, r.RuleID, r.Reason)
	}
}

// TestDispatchCommand_RmSplitFlags:rm -r -f / 无 git 上下文,filesystem 解析器应判 Deny。
func TestDispatchCommand_RmSplitFlags(t *testing.T) {
	r := DispatchCommand("rm -r -f /")
	if r.Decision != Deny {
		t.Errorf("rm -r -f /: got %v want Deny", r.Decision)
	}
	if r.RuleID != "filesystem.rm-rf-root-home" {
		t.Errorf("RuleID = %q, want filesystem.rm-rf-root-home", r.RuleID)
	}
}

// TestDispatchCommand_Snowflake:database 域命令应被 snowflake 解析器捕获。
func TestDispatchCommand_Snowflake(t *testing.T) {
	r := DispatchCommand("snow sql --query 'DROP TABLE x'")
	if r.Decision != Deny {
		t.Errorf("snow DROP TABLE: got %v want Deny", r.Decision)
	}
}

// TestDispatchCommand_Unknown:无语义解析器命中的命令返回 Unknown(交回正则)。
func TestDispatchCommand_Unknown(t *testing.T) {
	r := DispatchCommand("docker rm -f x")
	if r.Decision != Unknown {
		t.Errorf("docker 命令无解析器: got %v want Unknown", r.Decision)
	}
	r2 := DispatchCommand("ls -la")
	if r2.Decision != Unknown {
		t.Errorf("ls 命令无破坏性语义: got %v want Unknown", r2.Decision)
	}
}
