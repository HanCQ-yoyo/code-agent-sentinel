package main

import (
	"os"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/config"
)

func TestResolveAccessMethodLoopback(t *testing.T) {
	home := t.TempDir()
	am := resolveAccessMethod("127.0.0.1", 8080, home)
	if am.URL == "" || !contains(am.URL, "127.0.0.1:8080") {
		t.Errorf("URL: %+v", am)
	}
	if am.TunnelCmd == "" {
		t.Errorf("loopback 应给隧道命令: %+v", am)
	}
}

func TestResolveAccessMethodNonLoopback(t *testing.T) {
	am := resolveAccessMethod("0.0.0.0", 8080, "")
	if !contains(am.URL, "0.0.0.0:8080") {
		t.Errorf("URL: %+v", am)
	}
	if am.TunnelCmd != "" {
		t.Errorf("非 loopback 不应给隧道命令")
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || (len(s) > 0 && indexOf(s, sub) >= 0)) }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestMainWritesNothingOnHelp(t *testing.T) {
	// 确保 cobra 注册不 panic
	if err := newRootCmd().Help(); err != nil {
		t.Fatal(err)
	}
	_ = os.Stdout
	_ = filepath.Base
}

// TestTokenFlagRegistered 验证 C-BUILD-1:--token flag 已注册。
// 前端 e2e 依赖 --token 传入已知 token;flag 缺失会令 e2e 无法认证。
func TestTokenFlagRegistered(t *testing.T) {
	cmd := newRootCmd()
	flag := cmd.Flag("token")
	if flag == nil {
		t.Fatal("--token flag 未注册")
	}
	if flag.DefValue != "" {
		t.Errorf("--token 默认值应为空(随机 token),got %q", flag.DefValue)
	}
}

// TestClaudeDirFlagRegistered 验证 Task 3:--claude-dir flag 已注册,默认空。
// run() 在 flag 非空时覆盖 cfg.ResolveClaudeDir(home),空则走配置/默认回退。
func TestClaudeDirFlagRegistered(t *testing.T) {
	cmd := newRootCmd()
	f := cmd.Flags().Lookup("claude-dir")
	if f == nil {
		t.Fatal("--claude-dir flag 未注册")
	}
	if f.DefValue != "" {
		t.Errorf("--claude-dir 默认应空,got %q", f.DefValue)
	}
}

// TestShouldPromptSetupWhenUnconfigured 验证 Task 8:Agents 空 + ClaudeDir 空 +
// 默认 ~/.claude 不存在时,run() 应打印 setup 提示。抽 shouldPromptSetup 纯函数单测
// (run() 启动真实 HTTP server 阻塞,无法直接测)。
func TestShouldPromptSetupWhenUnconfigured(t *testing.T) {
	cfg := &config.Config{} // Agents 与 ClaudeDir 都空
	if !shouldPromptSetup(cfg, "/nonexistent/.claude") {
		t.Error("Agents 空 + .claude 不存在应提示 setup")
	}
}

// TestShouldNotPromptWhenClaudeDirSet 验证:用户显式配置 claude_dir(回退路径)
// 不应提示——属正常默认。
func TestShouldNotPromptWhenClaudeDirSet(t *testing.T) {
	cfg := &config.Config{ClaudeDir: "/some/.claude"}
	if shouldPromptSetup(cfg, "/some/.claude") {
		t.Error("ClaudeDir 非空(回退路径)不应提示")
	}
}

// TestShouldNotPromptWhenClaudeDirExists 验证:默认 ~/.claude 已存在(回退路径)
// 不应提示——属正常默认。
func TestShouldNotPromptWhenClaudeDirExists(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	cfg := &config.Config{}
	if shouldPromptSetup(cfg, filepath.Join(home, ".claude")) {
		t.Error("默认 .claude 存在不应提示")
	}
}
