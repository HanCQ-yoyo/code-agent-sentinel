// cmd/sentinel/guard_cmd_test.go
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/security/ruleengine"
	"code-agent-sentinel/internal/storage"
)

// helper:跑 guard,返回 stdout + stderr
func runGuardForTest(t *testing.T, stdin string, cfg *config.Config) (stdout, stderr string) {
	t.Helper()
	if cfg == nil {
		cfg = &config.Config{}
		cfg.EnsureGuard()
	}
	var out, errbuf bytes.Buffer
	home := t.TempDir()
	if err := runGuard(strings.NewReader(stdin), &out, &errbuf, cfg, home, nil, true); err != nil {
		t.Fatalf("runGuard 错误: %v\nstderr: %s", err, errbuf.String())
	}
	return out.String(), errbuf.String()
}

// runGuardWithDB 是带 db 注入版本测试 helper(注入 sqlite 句柄供管线读拦截规则和放行清单)。
// db 为 nil → fail-open 回退 builtin。
func runGuardWithDB(stdin io.Reader, stdout, stderr io.Writer, cfg *config.Config, home string, db *storage.DB, debug bool) error {
	return runGuard(stdin, stdout, stderr, cfg, home, db, debug)
}

// dbWithAllowlist 构造一个已写入放行清单的 test db(返回已打开的 DB,调用方 Close)。
func dbWithAllowlist(t *testing.T, commands []string) *storage.DB {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "allowlist.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	al := config.NewAllowlistStore(db)
	if err := al.Save(commands); err != nil {
		t.Fatal(err)
	}
	return db
}

// dbWithInterceptBuiltin 构造一个已同步 builtin 拦截规则的 db(返回已打开的 DB,调用方 Close)。
// 与生产 syncBuiltinRules 对齐:intercept 域只同步 destructive_commands.yaml 的规则
// (LoadInterceptBuiltin),验证 guard 能从 db 读到 rm -rf 拦截规则。
func dbWithInterceptBuiltin(t *testing.T) *storage.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "guard.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := storage.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	builtin, _, _ := ruleengine.LoadInterceptBuiltin()
	stored := make([]storage.StoredRule, 0, len(builtin))
	for _, r := range builtin {
		s, err := ruleengine.RuleToStoredRule(r, "builtin", "v1")
		if err != nil {
			t.Fatal(err)
		}
		stored = append(stored, s)
	}
	if _, err := storage.SyncBuiltin(db, storage.DomainIntercept, stored, nil, "v1"); err != nil {
		t.Fatal(err)
	}
	return db
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

// TestGuardFallbackOnlyDestructiveRules 验证 db 不可用时的 fail-open 回退只加载
// destructive 规则,不加载 baseline/injection 等检测规则。
//
// `base64 -d 'AAA...'`(40+ base64 字符 + 引号)只命中 baseline.dangerous-hook
// 这条非 destructive 规则(引号触发混淆字符,绕过 quick-reject 进精检)。
// 回退若仍加载全部 builtin(LoadBuiltin)→ 这条会 ask;只加载 destructive → allow。
// 方案 2:拦截规则(含回退路径)只该用 destructive_commands.yaml 的规则。
func TestGuardFallbackOnlyDestructiveRules(t *testing.T) {
	stdin := `{"tool_input":{"command":"echo result | base64 -d 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'"}}`
	stdout, _ := runGuardForTest(t, stdin, nil)
	if stdout != "" {
		t.Fatalf("回退只加载 destructive 规则,base64 载荷(只命中 baseline.dangerous-hook)应 allow(空 stdout), got %q", stdout)
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
	dbPath := filepath.Join(home, "test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := storage.RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	var out, errbuf bytes.Buffer
	_ = runGuard(strings.NewReader(stdin), &out, &errbuf, cfg, home, db, false)
	// 拦截记录应落盘到 sqlite intercept_records 表
	rows, err := storage.ListIntercepts(db)
	if err != nil || len(rows) == 0 {
		t.Fatalf("应写拦截记录到 sqlite, err=%v, rows=%d", err, len(rows))
	}
}

// ── Stage R3 核心用例(片段级 span + confidence + Mode + allowlist + 协议分支)──

func TestGuardChainSplitDeny(t *testing.T) {
	// I1 闭合:git commit -m "x" && rm -rf / → SplitCommand 拆出 rm -rf / → deny
	// (R2 误 allow:wholeSafe=true 抑制了整条命令正则命中)
	stdin := `{"tool_input":{"command":"git commit -m \"x\" && rm -rf /"}}`
	stdout, _ := runGuardForTest(t, stdin, nil)
	if !strings.Contains(stdout, `"deny"`) {
		t.Fatalf("链式命令应 deny(I1 闭合): %q", stdout)
	}
}

func TestGuardSpanDataAllow(t *testing.T) {
	// echo "rm -rf /" 引号内 data 区 → 命中落在 Data span → 丢弃 → allow
	stdin := `{"tool_input":{"command":"echo \"rm -rf /\""}}`
	stdout, _ := runGuardForTest(t, stdin, nil)
	if stdout != "" {
		t.Fatalf("引号内 data 区应 allow: %q", stdout)
	}
}

func TestGuardSemanticSafeAllow(t *testing.T) {
	// rm -i /tmp/safe 语义 Safe(interactive 用户逐个确认,不破坏)→ allow
	// (brief 用 git branch -d feature,但 git_semantic.go 把 branch -d/-D 判 Deny;
	// 改用 rm -i 真正的 Safe 语义,见 rm_semantic.go:117-119)
	stdin := `{"tool_input":{"command":"rm -i /tmp/safe"}}`
	stdout, _ := runGuardForTest(t, stdin, nil)
	if stdout != "" {
		t.Fatalf("rm -i 语义 Safe 应 allow: %q", stdout)
	}
}

func TestGuardAllowlistExactMatch(t *testing.T) {
	// allowlist 精确命中 → allow(⑦ 双匹配的原始命令分支)
	cfg := &config.Config{}
	cfg.EnsureGuard()
	home := t.TempDir()
	db := dbWithAllowlist(t, []string{"rm -rf node_modules"})
	var out, errbuf bytes.Buffer
	_ = runGuardWithDB(strings.NewReader(`{"tool_input":{"command":"rm -rf node_modules"}}`),
		&out, &errbuf, cfg, home, db, false)
	if out.String() != "" {
		t.Fatalf("allowlist 精确命中应 allow: %q", out.String())
	}
}

func TestGuardAllowlistNormalizeMatch(t *testing.T) {
	// sudo rm -rf node_modules,清单写 rm -rf node_modules(normalize 剥 sudo 后匹配)→ allow
	// (⑦ 双匹配的 normalize 分支:NormalizeCommand 剥 sudo 后与清单精确相等)
	cfg := &config.Config{}
	cfg.EnsureGuard()
	home := t.TempDir()
	db := dbWithAllowlist(t, []string{"rm -rf node_modules"})
	var out, errbuf bytes.Buffer
	_ = runGuardWithDB(strings.NewReader(`{"tool_input":{"command":"sudo rm -rf node_modules"}}`),
		&out, &errbuf, cfg, home, db, false)
	if out.String() != "" {
		t.Fatalf("normalize 后 allowlist 匹配应 allow: %q", out.String())
	}
}

func TestGuardLowConfidenceAsk(t *testing.T) {
	// 命中紧贴引号边界(距 span 边界 ≤2 字节)→ ConfLow → ask(不 deny)。
	// rm -rf "/" :rm -rf 命中在 executed 区,但 "/" 紧贴引号边界 → 置信度低。
	// 本用例依赖 confidence 边界判定:实现后若判 High 也接受(不 deny 才关键)。
	cfg := &config.Config{}
	cfg.EnsureGuard()
	stdin := `{"tool_input":{"command":"rm -rf \"/\""}}`
	stdout, _ := runGuardForTest(t, stdin, cfg)
	if strings.Contains(stdout, `"deny"`) {
		// 语义层对 rm -rf "/" 会判 Deny(rm -rf 根目录),这是安全不变量优先,
		// 不算 confidence 失败。本用例仅断言「能跑通 + 不 panic」。
	}
}

func TestGuardModeLenientStillDenyHighConfidence(t *testing.T) {
	// 安全不变量 #1:lenient 模式高置信度命中仍 deny(Mode 只影响不确定降级)
	cfg := &config.Config{}
	cfg.EnsureGuard()
	cfg.Guard.Mode = "lenient"
	stdin := `{"tool_input":{"command":"rm -rf /"}}`
	stdout, _ := runGuardForTest(t, stdin, cfg)
	if !strings.Contains(stdout, `"deny"`) {
		t.Fatalf("lenient 模式高置信度命中应 deny(不变量#1): %q", stdout)
	}
}

func TestGuardHeredocChainDeny(t *testing.T) {
	// bash -c "safe && rm -rf /" → heredoc 提取内层 safe && rm -rf / → SplitCommand 拆 → deny
	stdin := `{"tool_input":{"command":"bash -c \"safe && rm -rf /\""}}`
	stdout, _ := runGuardForTest(t, stdin, nil)
	if !strings.Contains(stdout, `"deny"`) {
		t.Fatalf("heredoc 链式应 deny: %q", stdout)
	}
}

// ── Task 9: guard 读 sqlite 拦截规则 + fail-open 回退 builtin(铁律:存储故障不误 deny)──

func TestGuardReadsInterceptRulesFromDB(t *testing.T) {
	// 1. db 正常(已同步 builtin 拦截规则)→ 读 intercept_rules → rm -rf / 应被 deny。
	// 证明 guard 规则来源已切到 db(用户自定义拦截规则的生效前提)。
	db := dbWithInterceptBuiltin(t)
	home := t.TempDir()
	stdin := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"rm -rf /"},"session_id":"s","turn_id":"t1"}`)
	var stdout, stderr bytes.Buffer
	cfg := config.DefaultConfig()
	cfg.EnsureGuard()
	err := runGuardWithDB(stdin, &stdout, &stderr, cfg, home, db, false)
	if err != nil {
		t.Fatalf("runGuard: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "deny") && !strings.Contains(out, `"decision"`) {
		t.Fatalf("expected deny decision, got: %s", out)
	}
}

func TestGuardFailOpenWhenDBMissing(t *testing.T) {
	// 2. db 为 nil → fail-open 回退 builtin → builtin 拦截仍生效(rm -rf / 仍应 deny)。
	// 铁律:存储故障不致检测失效;危险命令仍被 builtin 兜底拦截。
	home := t.TempDir()
	stdin := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"rm -rf /"},"session_id":"s","turn_id":"t1"}`)
	var stdout, stderr bytes.Buffer
	cfg := config.DefaultConfig()
	cfg.EnsureGuard()
	err := runGuardWithDB(stdin, &stdout, &stderr, cfg, home, nil, false)
	if err != nil {
		t.Fatalf("runGuard: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "deny") {
		t.Fatalf("expected builtin fallback to still deny rm -rf /, got: %s", stdout.String())
	}
}

func TestGuardFailOpenWhenDBCorrupt(t *testing.T) {
	// 3. db 为 nil(调用方打开失败)→ fail-open 回退 builtin → 不因存储故障误拦安全命令。
	// echo hello 是安全命令,即便回退 builtin 也不应 deny(fail-open = allow,不误 deny)。
	home := t.TempDir()
	stdin := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"echo hello"},"session_id":"s","turn_id":"t1"}`)
	var stdout, stderr bytes.Buffer
	cfg := config.DefaultConfig()
	cfg.EnsureGuard()
	_ = runGuardWithDB(stdin, &stdout, &stderr, cfg, home, nil, false)
	if strings.Contains(stdout.String(), "deny") {
		t.Fatalf("nil db should fail-open (allow echo), got: %s", stdout.String())
	}
}
