package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/security/findingstate"
	"code-agent-sentinel/internal/storage"
)

// TestRulesValidateReportsInvalid 验证 sentinel rules validate <file> 能检出非法 op。
// 写一个 match.op=bogus 的规则文件,跑校验,断言输出含 "bogus"。
func TestRulesValidateReportsInvalid(t *testing.T) {
	tmp := t.TempDir()
	badFile := filepath.Join(tmp, "bad.yaml")
	badYAML := "rules:\n  - id: bad-rule\n    severity: high\n    asset_type: settings\n    match:\n      field: fields.allow\n      op: bogus\n      value: \"foo\"\n"
	if err := os.WriteFile(badFile, []byte(badYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := execRulesValidate(tmp, badFile)
	if err == nil {
		t.Fatal("含非法规则的 validate 应返回 error")
	}
	if !strings.Contains(out, "bogus") {
		t.Fatalf("validate 应报告 bogus op: %s", out)
	}
}

// TestRulesValidateValidFile 验证合法规则文件校验通过(无错误)。
func TestRulesValidateValidFile(t *testing.T) {
	tmp := t.TempDir()
	goodFile := filepath.Join(tmp, "good.yaml")
	goodYAML := "rules:\n  - id: good-rule\n    severity: high\n    asset_type: settings\n    match:\n      field: fields.allow\n      op: contains\n      value: \"Bash(*)\"\n    description: \"测试规则\"\n    remediation: \"修复\"\n"
	if err := os.WriteFile(goodFile, []byte(goodYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := execRulesValidate(tmp, goodFile)
	if err != nil {
		t.Fatalf("合法规则不应返回 error: %v, out=%s", err, out)
	}
	if !strings.Contains(out, "good-rule") {
		t.Fatalf("validate 输出应含规则 id: %s", out)
	}
	if !strings.Contains(out, "有效规则") {
		t.Fatalf("validate 输出应标记有效规则数: %s", out)
	}
}

// TestRulesListShowsBuiltin 验证 sentinel rules list 列出内置规则。
func TestRulesListShowsBuiltin(t *testing.T) {
	home := t.TempDir()
	out, err := execRulesList(home)
	if err != nil {
		t.Fatalf("rules list error: %v", err)
	}
	// 内置规则含 baseline.wildcard-bash(11 条 baseline 之一)
	if !strings.Contains(out, "baseline.wildcard-bash") {
		t.Fatalf("rules list 应含内置规则 baseline.wildcard-bash: %s", out)
	}
	// 应含表头
	if !strings.Contains(out, "ID") || !strings.Contains(out, "SEVERITY") {
		t.Fatalf("rules list 应含表头: %s", out)
	}
}

// TestRulesListReadsFromDB 验证 rules list 优先读 sqlite(Task 13):
// 建 db + 同步 builtin + 插一条 custom 规则,断言 list 输出含该 custom 规则
// (fail-open 路径只显 builtin,显出 custom 即证走的是 db 路径)。
func TestRulesListReadsFromDB(t *testing.T) {
	home := t.TempDir()
	// writeTestConfig 会建 ~/.claude-sentinel/config.yaml,但 db 路径是
	// <home>/.claude-sentinel/sentinel.db(与 main.go 启动一致)。先建目录+建 db:
	sentinelDir := filepath.Join(home, ".claude-sentinel")
	if err := os.MkdirAll(sentinelDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	dbPath := filepath.Join(sentinelDir, "sentinel.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := storage.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	// 同步 builtin(模拟 main.go 启动),再插一条 custom 规则。
	syncBuiltinRules(db)
	custom := storage.StoredRule{
		ID: "custom.cli-list-probe", Severity: "high", AssetType: "command",
		MatchJSON: `{"field":"command","op":"contains","value":"cli-list-probe"}`,
		Source: "custom",
	}
	if err := storage.UpsertRule(db, storage.DomainDetect, "custom", custom, ""); err != nil {
		t.Fatalf("UpsertRule custom: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close: %v", err)
	}

	out, err := execRulesList(home)
	if err != nil {
		t.Fatalf("rules list error: %v", err)
	}
	// custom 规则应可见(证明读的是 db,非 fail-open builtin)
	if !strings.Contains(out, "custom.cli-list-probe") {
		t.Fatalf("rules list 应从 db 读到 custom 规则 custom.cli-list-probe: %s", out)
	}
	// builtin 也应在(db 同步了 builtin 行)
	if !strings.Contains(out, "baseline.wildcard-bash") {
		t.Fatalf("rules list 应从 db 读到 builtin baseline.wildcard-bash: %s", out)
	}
}

// TestBaselineCreate 验证 sentinel baseline --create 能跑全量扫描并把指纹批量接受到 finding_states.yaml。
// Task 11 语义变更:旧实现 union 到 baseline.json(已删);新实现调 BulkAccept 写 finding_states.yaml。
// BulkAccept 不覆盖已有非 open 状态:预置一条 resolved 状态,验证 --create 后仍为 resolved(不被 accepted 覆盖)。
func TestBaselineCreate(t *testing.T) {
	home := t.TempDir()
	// 构造一个会触发 baseline.wildcard-bash 的 settings.json
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsJSON := `{"permissions":{"allow":["Bash(*)"]}}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	statesPath := cfg.ResolveStatesPath(home)

	// 预置一条不会在本次扫描复现的假指纹(resolved 状态,模拟之前已处置的旧 finding)
	preseed := &findingstate.States{Items: []findingstate.State{
		{Fingerprint: "preseed-resolved-fp", Status: findingstate.StatusResolved, Note: "之前已修复"},
	}}
	if err := preseed.Save(statesPath); err != nil {
		t.Fatalf("预置 finding_states 失败: %v", err)
	}

	out, err := runBaselineCreate(cfg, home)
	if err != nil {
		t.Fatalf("baseline create error: %v\noutput: %s", err, out)
	}
	// finding_states.yaml 应存在
	if _, err := os.ReadFile(statesPath); err != nil {
		t.Fatalf("finding_states.yaml 应存在: %v", err)
	}

	// 验证:预置的 resolved 状态保留(不被 BulkAccept 覆盖为 accepted)
	loaded, err := findingstate.Load(statesPath)
	if err != nil {
		t.Fatalf("加载 finding_states 失败: %v", err)
	}
	if loaded == nil {
		t.Fatal("finding_states 为空")
	}
	var preseedKept bool
	var acceptCount int
	for _, it := range loaded.Items {
		if it.Fingerprint == "preseed-resolved-fp" {
			preseedKept = true
			if it.Status != findingstate.StatusResolved {
				t.Errorf("预置 resolved 状态被覆盖为 %s(BulkAccept 不应覆盖非 open)", it.Status)
			}
		}
		if it.Status == findingstate.StatusAccepted {
			acceptCount++
		}
	}
	if !preseedKept {
		t.Error("预置的 resolved 状态被删除(应保留)")
	}
	if acceptCount == 0 {
		t.Error("应至少有 1 条 accepted(baseline.wildcard-bash 命中 Bash(*))")
	}
}

// TestBaselinePrune 验证 sentinel baseline --prune 删除已不复现的孤儿状态。
func TestBaselinePrune(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsJSON := `{"permissions":{"allow":["Bash(*)"]}}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	statesPath := cfg.ResolveStatesPath(home)

	// (1) 先 create 生成 finding_states(含 baseline.wildcard-bash 的 fingerprint)
	if _, err := runBaselineCreate(cfg, home); err != nil {
		t.Fatalf("baseline create error: %v", err)
	}
	st, err := findingstate.Load(statesPath)
	if err != nil {
		t.Fatalf("加载 finding_states 失败: %v", err)
	}
	if len(st.Items) == 0 {
		t.Fatal("baseline create 应产出处置状态")
	}
	countBefore := len(st.Items)

	// (2) 塞一条假指纹(模拟已不复现的旧 finding,accepted 状态)
	st.Set("fake-stale-fingerprint", findingstate.State{Status: findingstate.StatusAccepted, Source: findingstate.SourceManual})
	if err := st.Save(statesPath); err != nil {
		t.Fatal(err)
	}

	// (3) prune:应删掉假指纹(本轮未检出),保留真实指纹
	if _, err := runBaselinePrune(cfg, home); err != nil {
		t.Fatalf("baseline prune error: %v", err)
	}
	st2, err := findingstate.Load(statesPath)
	if err != nil {
		t.Fatalf("加载 pruned finding_states 失败: %v", err)
	}
	if len(st2.Items) != countBefore {
		t.Fatalf("prune 后应剩 %d 条(=复现的), got %d", countBefore, len(st2.Items))
	}
	for _, it := range st2.Items {
		if it.Fingerprint == "fake-stale-fingerprint" {
			t.Fatalf("prune 应删除假指纹,但仍存在: %s", it.Fingerprint)
		}
	}
}

// TestRulesCmdRegistered 验证 rules 子命令已注册到 root。
func TestRulesCmdRegistered(t *testing.T) {
	root := newRootCmd()
	for _, c := range root.Commands() {
		if c.Use == "rules" {
			// 检查子命令
			hasList, hasValidate := false, false
			for _, sub := range c.Commands() {
				if sub.Use == "list" {
					hasList = true
				}
				if strings.HasPrefix(sub.Use, "validate") {
					hasValidate = true
				}
			}
			if !hasList {
				t.Error("rules 子命令缺少 list")
			}
			if !hasValidate {
				t.Error("rules 子命令缺少 validate")
			}
			return
		}
	}
	t.Fatal("root 缺少 rules 子命令")
}

// TestBaselineCmdRegistered 验证 baseline 子命令已注册到 root。
func TestBaselineCmdRegistered(t *testing.T) {
	root := newRootCmd()
	for _, c := range root.Commands() {
		if c.Use == "baseline" {
			if c.Flags().Lookup("create") == nil {
				t.Error("baseline 子命令缺少 --create flag")
			}
			if c.Flags().Lookup("prune") == nil {
				t.Error("baseline 子命令缺少 --prune flag")
			}
			return
		}
	}
	t.Fatal("root 缺少 baseline 子命令")
}

// --- 测试 helper ---

// execRulesList 执行 `sentinel rules list`,返回 stdout。homeDir 通过临时 config 文件注入。
func execRulesList(home string) (string, error) {
	cfgPath := writeTestConfig(home)
	root := newRootCmd()
	var buf strings.Builder
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"rules", "list", "--config", cfgPath})
	err := root.Execute()
	return buf.String(), err
}

// execRulesValidate 执行 `sentinel rules validate <file>`,返回 stdout。
func execRulesValidate(home, file string) (string, error) {
	cfgPath := writeTestConfig(home)
	root := newRootCmd()
	var buf strings.Builder
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"rules", "validate", file, "--config", cfgPath})
	err := root.Execute()
	return buf.String(), err
}

// writeTestConfig 在 home/.claude-sentinel/config.yaml 写一份最小配置(设 home_dir=home),
// 返回路径。让 loadCfgAndHome 用此 config 解析到正确的 home。
// home 必须非空(否则 filepath.Join 退化成相对路径,会在 cwd 泄漏创建 .claude-sentinel/)。
func writeTestConfig(home string) string {
	if home == "" {
		panic("writeTestConfig: home must be non-empty to avoid leaking .claude-sentinel/ into cwd")
	}
	dir := filepath.Join(home, ".claude-sentinel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}
	cfgPath := filepath.Join(dir, "config.yaml")
	content := "home_dir: " + home + "\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		panic(err)
	}
	return cfgPath
}
