package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

// TestServeHTTPGracefulShutdown:触发 shutdownTrigger 后 Serve 返回且 stop 被调用。
// Task 18:验证 serveHTTP 抽出函数——shutdownTrigger 关闭 → srv.Shutdown → Serve 返回
// http.ErrServerClosed(转 nil)→ stop 回调被调用(生产里即 mgr.Stop)。
func TestServeHTTPGracefulShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })}
	var stopCalled int32
	stop := func() { atomic.StoreInt32(&stopCalled, 1) }

	trig := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- serveHTTP(ln, srv, stop, trig) }()

	time.Sleep(50 * time.Millisecond) // 让 Serve 起来
	close(trig)                        // 触发 shutdown

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serveHTTP 应返回 nil: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serveHTTP 未在 3s 内返回")
	}
	if atomic.LoadInt32(&stopCalled) != 1 {
		t.Error("stop(mgr.Stop) 应被调用")
	}
}

// TestLogPathFlag 验证 --log-path 标准库烟雾测试:OpenFile + log.SetOutput + 写入 + 读回。
// 不调用 run()(run 会起真实 HTTP server 阻塞);此处只校验 log.SetOutput 路径可写。
func TestLogPathFlag(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")
	// 模拟 --log-path flag 落地后的 run() 逻辑片段
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	log.SetOutput(f)
	log.Println("test log entry")
	f.Close()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("日志文件未创建: %v", err)
	}
	if !strings.Contains(string(data), "test log entry") {
		t.Error("日志文件应包含写入内容")
	}
	// 恢复
	log.SetOutput(os.Stderr)
}

// TestLogPathConfigFallback 验证 config.yaml 的 log_path 字段能被 Load 读到 cfg.LogPath。
// --log-path flag > cfg.LogPath > 默认 stderr,run() 里用 cfg.LogPath 兜底。
func TestLogPathConfigFallback(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	// 写带 log_path 的 config(路径任意,不要求文件存在;Load 不校验)
	if err := os.WriteFile(cfgPath, []byte("log_path: /tmp/sentinel-test.log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LogPath != "/tmp/sentinel-test.log" {
		t.Errorf("LogPath 应从 config 读取: got %q", cfg.LogPath)
	}
}

// TestLogPathFlagRegistered 验证 --log-path flag 已注册,默认空(stderr)。
// service install 生成的单元会传 --log-path 指向日志文件;flag 缺失会令服务退回 stderr。
func TestLogPathFlagRegistered(t *testing.T) {
	cmd := newRootCmd()
	f := cmd.Flags().Lookup("log-path")
	if f == nil {
		t.Fatal("--log-path flag 未注册")
	}
	if f.DefValue != "" {
		t.Errorf("--log-path 默认应空(stderr),got %q", f.DefValue)
	}
}

// TestDaemonFlagParsed 验证 Task 15:--daemon flag 已注册并可解析。
// service install 生成的单元可带 --daemon 让 sentinel 脱离终端后台运行。
func TestDaemonFlagParsed(t *testing.T) {
	cmd := newRootCmd()
	if err := cmd.Flags().Parse([]string{"--daemon", "--no-browser", "--log-path", "/dev/null"}); err != nil {
		t.Fatalf("flag parse err: %v", err)
	}
	if !cmd.Flags().Changed("daemon") {
		t.Error("--daemon flag 应被识别")
	}
}

// TestDaemonFlagRegistered 验证 --daemon flag 已注册,默认 false。
func TestDaemonFlagRegistered(t *testing.T) {
	cmd := newRootCmd()
	f := cmd.Flags().Lookup("daemon")
	if f == nil {
		t.Fatal("--daemon flag 未注册")
	}
	if f.DefValue != "false" {
		t.Errorf("--daemon 默认应 false,got %q", f.DefValue)
	}
}

// TestDaemonChildFlagHidden 验证 --daemon-child 是 hidden flag(内部防重复 fork 标记,
// 不对用户暴露)。cobra 仍接受它(避免 unknown flag 错误),但 --help 不显示。
func TestDaemonChildFlagHidden(t *testing.T) {
	cmd := newRootCmd()
	f := cmd.Flags().Lookup("daemon-child")
	if f == nil {
		t.Fatal("--daemon-child flag 未注册(应 hidden 注册以让 cobra 接受)")
	}
	if !f.Hidden {
		t.Error("--daemon-child 应 MarkHidden(不对用户暴露)")
	}
}

// TestDaemonizeChild 验证 Task 15:--daemon-child 在 os.Args 中时,daemonize() 返回
// (true, nil)——当前进程是子进程,不再 fork(防递归)。只测 guard 路径,
// 不测真实 fork(真实 fork 会启动 sentinel 服务,属集成测试范畴)。
// 注意:此测试 mutate os.Args,不 t.Parallel();defer 恢复。
func TestDaemonizeChild(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = append(os.Args, "--daemon-child")
	child, err := daemonize()
	if err != nil {
		t.Fatalf("daemonize err: %v", err)
	}
	if !child {
		t.Error("--daemon-child 存在时应返回 child=true(当前进程是子进程,不再 fork)")
	}
}
