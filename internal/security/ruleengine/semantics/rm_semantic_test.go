package semantics

import "testing"

// TestRmSemantic_RootForce: rm -rf / 是破坏性,语义应识别为 Deny。
// 正则 rm-rf-root-home 能匹配,但语义层也应识别(双保险)。
func TestRmSemantic_RootForce(t *testing.T) {
	r := RmSemanticDecision("rm -rf /")
	if r.Decision != Deny {
		t.Errorf("rm -rf /: got %v want Deny (rule=%s)", r.Decision, r.RuleID)
	}
}

// TestRmSemantic_SplitFlagsForce: rm -r -f / —— flag 拆分,
// 正则 rm-rf-root-home 可能漏(只匹配 -rf 聚簇),语义应识别。
func TestRmSemantic_SplitFlagsForce(t *testing.T) {
	r := RmSemanticDecision("rm -r -f /")
	if r.Decision != Deny {
		t.Errorf("rm -r -f /: got %v want Deny (rule=%s)", r.Decision, r.RuleID)
	}
}

// TestRmSemantic_InteractiveSafe: rm -i 是 interactive,用户逐个确认,安全。
// 正则 rm-rf-general 可能误报(匹配 -i 中的 f? 不会,但其他正则可能),
// 语义应返回 Safe 抑制误报。
func TestRmSemantic_InteractiveSafe(t *testing.T) {
	r := RmSemanticDecision("rm -i file")
	if r.Decision != Safe {
		t.Errorf("rm -i: got %v want Safe", r.Decision)
	}
}

// TestRmSemantic_PipeStdinInteractive: echo y | rm -i —— 管道自动确认 -i,
// 绕过 interactive 安全屏障,变危险。正则可能漏(不含 -rf),
// 语义应识别 pipe stdin → Deny。
func TestRmSemantic_PipeStdinInteractive(t *testing.T) {
	r := RmSemanticDecision("echo y | rm -i file")
	if r.Decision != Deny {
		t.Errorf("echo y | rm -i: got %v want Deny(管道 stdin 自动确认)", r.Decision)
	}
}

// TestRmSemantic_DashDashFilename: rm -- -rf —— -- 后 -rf 是文件名,不是 flag。
// 正则 rm-rf-general 会误报(看到 -rf),语义应识别 -- 后非 flag → not Deny。
func TestRmSemantic_DashDashFilename(t *testing.T) {
	r := RmSemanticDecision("rm -- -rf")
	if r.Decision == Deny {
		t.Errorf("rm -- -rf: got Deny,但 -rf 是文件名不是 flag")
	}
}

// TestRmSemantic_NotRm: 非 rm 命令 → Unknown(交回正则层)。
func TestRmSemantic_NotRm(t *testing.T) {
	r := RmSemanticDecision("ls -la")
	if r.Decision != Unknown {
		t.Errorf("ls: got %v want Unknown", r.Decision)
	}
}

// --- 边界覆盖(brief 鼓励补 edge case)---

// TestRmSemantic_SudoWrapper: sudo rm -rf / 仍应识别为 Deny。
func TestRmSemantic_SudoWrapper(t *testing.T) {
	r := RmSemanticDecision("sudo rm -rf /")
	if r.Decision != Deny {
		t.Errorf("sudo rm -rf /: got %v want Deny (rule=%s)", r.Decision, r.RuleID)
	}
}

// TestRmSemantic_ClusterReordered: rm -fr / —— flag 聚簇重排(-fr 而非 -rf),
// 语义应识别 force+recursive。
func TestRmSemantic_ClusterReordered(t *testing.T) {
	r := RmSemanticDecision("rm -fr /")
	if r.Decision != Deny {
		t.Errorf("rm -fr /: got %v want Deny (rule=%s)", r.Decision, r.RuleID)
	}
}

// TestRmSemantic_NonRootForce: rm -rf /tmp/x —— 非根目录但 recursive+force,
// 仍是 Deny(rm-recursive-force,破坏性递归强制删除)。
func TestRmSemantic_NonRootForce(t *testing.T) {
	r := RmSemanticDecision("rm -rf /tmp/x")
	if r.Decision != Deny {
		t.Errorf("rm -rf /tmp/x: got %v want Deny (rule=%s)", r.Decision, r.RuleID)
	}
}

// TestRmSemantic_LongFlags: rm --recursive --force / —— 长 flag 形式。
// 回归 brief 的 char-scan bug:原实现扫描 --recursive 每个字符,
// 'r' 误设 recursive、'i' 误设 interactive(把 --recursive 当成含 r+i 的短聚簇)。
// 修正后应正确识别 recursive+force,且不误设 interactive。
func TestRmSemantic_LongFlags(t *testing.T) {
	r := RmSemanticDecision("rm --recursive --force /")
	if r.Decision != Deny {
		t.Errorf("rm --recursive --force /: got %v want Deny (rule=%s)", r.Decision, r.RuleID)
	}
}

// TestRmSemantic_LongInteractiveSafe: rm --interactive file —— 长 interactive flag,
// 应返回 Safe(用户逐个确认)。回归 brief 的 char-scan bug:
// 原实现扫描 --interactive 会误设 recursive(从 'r')。
func TestRmSemantic_LongInteractiveSafe(t *testing.T) {
	r := RmSemanticDecision("rm --interactive file")
	if r.Decision != Safe {
		t.Errorf("rm --interactive: got %v want Safe", r.Decision)
	}
}

// TestRmSemantic_PipeAfterRmNotStdin: rm file | grep —— 管道在 rm 之后,
// 是 rm 的输出到 grep,不是 rm 的 stdin。不应触发 pipe-stdin 检测。
// rm file 无 -r -f -i → Unknown。
func TestRmSemantic_PipeAfterRmNotStdin(t *testing.T) {
	r := RmSemanticDecision("rm file | grep")
	if r.Decision != Unknown {
		t.Errorf("rm file | grep: got %v want Unknown(pipe 是输出非输入)", r.Decision)
	}
}

// TestRmSemantic_EnvWrapper: env FOO=1 rm -rf / 应识别为 Deny。
func TestRmSemantic_EnvWrapper(t *testing.T) {
	r := RmSemanticDecision("env FOO=1 rm -rf /")
	if r.Decision != Deny {
		t.Errorf("env FOO=1 rm -rf /: got %v want Deny (rule=%s)", r.Decision, r.RuleID)
	}
}

// TestRmSemantic_HomeTarget: rm -rf ~ —— 删 home 目录,Deny。
func TestRmSemantic_HomeTarget(t *testing.T) {
	r := RmSemanticDecision("rm -rf ~")
	if r.Decision != Deny {
		t.Errorf("rm -rf ~: got %v want Deny (rule=%s)", r.Decision, r.RuleID)
	}
}

// TestRmSemantic_InteractiveNoPipeNotDeny: rm -i(无管道)→ Safe,
// rm -i file(无管道)→ Safe。已由 TestRmSemantic_InteractiveSafe 覆盖基本形式,
// 此用例补 sudo 前缀 + interactive。
func TestRmSemantic_SudoInteractiveSafe(t *testing.T) {
	r := RmSemanticDecision("sudo rm -i file")
	if r.Decision != Safe {
		t.Errorf("sudo rm -i: got %v want Safe(interactive 无管道)", r.Decision)
	}
}

// TestRmSemantic_NoArgs: rm(无参数)→ Unknown(无操作)。
func TestRmSemantic_NoArgs(t *testing.T) {
	r := RmSemanticDecision("rm")
	if r.Decision != Unknown {
		t.Errorf("rm: got %v want Unknown(无参数)", r.Decision)
	}
}

// TestRmSemantic_PipeNoSpaces: echo y|rm -i file —— 管道符两侧无空格。
// 回归 brief 的 strings.Index(command," rm ") 漏报:该写法无 " rm " 子串(rm 紧跟 |),
// 简易实现会漏报 pipe stdin → 误判 Safe。修正后用正则匹配位置,应识别 pipe → Deny。
func TestRmSemantic_PipeNoSpaces(t *testing.T) {
	r := RmSemanticDecision("echo y|rm -i file")
	if r.Decision != Deny {
		t.Errorf("echo y|rm -i: got %v want Deny(管道无空格也应识别 pipe stdin)", r.Decision)
	}
}

// TestRmSemantic_PipeSpaceBeforeOnly: echo y |rm -i file —— 管道符后无空格。
// 同样回归 brief 的 " rm " 子串漏报。
func TestRmSemantic_PipeSpaceBeforeOnly(t *testing.T) {
	r := RmSemanticDecision("echo y |rm -i file")
	if r.Decision != Deny {
		t.Errorf("echo y |rm -i: got %v want Deny(管道后无空格也应识别 pipe stdin)", r.Decision)
	}
}
