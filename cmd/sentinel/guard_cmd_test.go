// cmd/sentinel/guard_cmd_test.go
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code-agent-sentinel/internal/config"
)

// readDirNames 返回 dir 下文件名列表(忽略错误时返回 nil),供拦截记录落盘断言。
func readDirNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}

// helper:跑 guard,返回 stdout + stderr
func runGuardForTest(t *testing.T, stdin string, cfg *config.Config) (stdout, stderr string) {
	t.Helper()
	if cfg == nil {
		cfg = &config.Config{}
		cfg.EnsureGuard()
	}
	var out, errbuf bytes.Buffer
	home := t.TempDir()
	if err := runGuard(strings.NewReader(stdin), &out, &errbuf, cfg, home, true); err != nil {
		t.Fatalf("runGuard 错误: %v\nstderr: %s", err, errbuf.String())
	}
	return out.String(), errbuf.String()
}

func TestGuardDenyRmRfRoot(t *testing.T) {
	stdin := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"rm -rf /"}}`
	stdout, _ := runGuardForTest(t, stdin, nil)
	if stdout == "" {
		t.Fatal("rm -rf / 应 deny,stdout 不应为空")
	}
	var got map[string]map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &got); err != nil {
		t.Fatalf("deny 输出非 JSON: %v\n%s", err, stdout)
	}
	if got["hookSpecificOutput"]["permissionDecision"] != "deny" {
		t.Fatalf("应 deny, got %v", got["hookSpecificOutput"]["permissionDecision"])
	}
}

func TestGuardDenyGitResetHard(t *testing.T) {
	stdin := `{"tool_name":"Bash","tool_input":{"command":"git reset --hard"}}`
	stdout, _ := runGuardForTest(t, stdin, nil)
	if !strings.Contains(stdout, `"deny"`) {
		t.Fatalf("git reset --hard 应 deny, got %q", stdout)
	}
}

func TestGuardDenySudoRmRf(t *testing.T) {
	stdin := `{"tool_input":{"command":"sudo rm -rf /"}}`
	stdout, _ := runGuardForTest(t, stdin, nil)
	if !strings.Contains(stdout, `"deny"`) {
		t.Fatalf("sudo rm -rf / 应 deny(normalize 剥 sudo 后命中), got %q", stdout)
	}
}

func TestGuardDenyAnsiC(t *testing.T) {
	stdin := `{"tool_input":{"command":"$'\\x72\\x6d' -rf /"}}`
	stdout, _ := runGuardForTest(t, stdin, nil)
	if !strings.Contains(stdout, `"deny"`) {
		t.Fatalf("ANSI-C 编码 rm 应 deny, got %q", stdout)
	}
}

func TestGuardDenyBashCInline(t *testing.T) {
	stdin := `{"tool_input":{"command":"bash -c \"rm -rf /\""}}`
	stdout, _ := runGuardForTest(t, stdin, nil)
	if !strings.Contains(stdout, `"deny"`) {
		t.Fatalf("bash -c rm -rf / 应 deny(heredoc 提取), got %q", stdout)
	}
}

func TestGuardAllowSafeCommand(t *testing.T) {
	stdin := `{"tool_input":{"command":"ls -la"}}`
	stdout, _ := runGuardForTest(t, stdin, nil)
	if stdout != "" {
		t.Fatalf("ls -la 应 allow(空 stdout), got %q", stdout)
	}
}

func TestGuardAllowGitCommitMessage(t *testing.T) {
	// git commit -m "rm -rf" 数据区字面量,语义 Safe → 不 deny(关卡1 防误报)
	stdin := `{"tool_input":{"command":"git commit -m \"rm -rf\""}}`
	stdout, _ := runGuardForTest(t, stdin, nil)
	if strings.Contains(stdout, `"deny"`) {
		t.Fatalf("git commit -m 数据区字面量不应 deny, got %q", stdout)
	}
}

func TestGuardAllowQuickRejectLs(t *testing.T) {
	// ls 不含任何关键词 → quick-reject 放行(空 stdout)
	stdin := `{"tool_input":{"command":"echo hello world"}}`
	stdout, _ := runGuardForTest(t, stdin, nil)
	if stdout != "" {
		t.Fatalf("echo hello 应 quick-reject 放行, got %q", stdout)
	}
}

func TestGuardRecursiveShortCircuit(t *testing.T) {
	// sentinel guard 自身命令 → 短路 allow(防递归)
	stdin := `{"tool_input":{"command":"sentinel guard --debug"}}`
	stdout, _ := runGuardForTest(t, stdin, nil)
	if stdout != "" {
		t.Fatalf("sentinel guard 自身应短路 allow, got %q", stdout)
	}
}

func TestGuardFailOpenBadJSON(t *testing.T) {
	// 损坏 JSON → fail-open allow(空 stdout)
	stdout, _ := runGuardForTest(t, "not json {{{", nil)
	if stdout != "" {
		t.Fatalf("损坏 JSON 应 fail-open allow, got %q", stdout)
	}
}

func TestGuardDisabledAllowsAll(t *testing.T) {
	cfg := &config.Config{}
	cfg.EnsureGuard()
	cfg.Guard.Enabled = false
	stdin := `{"tool_input":{"command":"rm -rf /"}}`
	stdout, _ := runGuardForTest(t, stdin, cfg)
	if stdout != "" {
		t.Fatalf("guard disabled 应 allow 全部, got %q", stdout)
	}
}

func TestGuardWritesInterceptRecord(t *testing.T) {
	stdin := `{"tool_input":{"command":"rm -rf /"},"session_id":"ses-x","cwd":"/tmp"}`
	cfg := &config.Config{}
	cfg.EnsureGuard()
	home := t.TempDir()
	var out, errbuf bytes.Buffer
	_ = runGuard(strings.NewReader(stdin), &out, &errbuf, cfg, home, false)
	// 拦截记录应落盘 ~/.claude-sentinel/intercept/*.json
	dir := filepath.Join(home, ".claude-sentinel", "intercept")
	entries, err := readDirNames(dir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("应写拦截记录到 %s, err=%v", dir, err)
	}
}
