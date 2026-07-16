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

// TestResolveSchedulerInterval 验证 Task 10:定时扫描配置解析。
// run() 启动真实 HTTP server 难以单测,故抽 resolveSchedulerInterval 纯函数单测。
// 总开关关 / 间隔空 / 无效 → (0, false);否则 (interval, true)。
func TestResolveSchedulerInterval(t *testing.T) {
	cases := []struct {
		enabled  bool
		interval string
		wantDur  string
		wantEn   bool
	}{
		{false, "30m", "0s", false},  // 总开关关 → 不启用
		{true, "", "0s", false},      // 间隔空 → 不启用
		{true, "bad", "0s", false},   // 无效 → 不启用
		{true, "30m", "30m0s", true}, // 正常
		{true, "1h", "1h0m0s", true},
	}
	for _, c := range cases {
		cfg := &config.Config{ScanEnabled: c.enabled, ScanInterval: c.interval}
		dur, en := resolveSchedulerInterval(cfg)
		if en != c.wantEn {
			t.Errorf("enabled=%v interval=%q: want en=%v got %v", c.enabled, c.interval, c.wantEn, en)
		}
		if c.wantEn && dur.String() != c.wantDur {
			t.Errorf("interval=%q: want dur=%q got %q", c.interval, c.wantDur, dur)
		}
	}
}
