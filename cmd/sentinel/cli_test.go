package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/security/suppression"
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

// TestBaselineCreate 验证 sentinel baseline --create 能跑全量扫描并把指纹写入 baseline.json。
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
	baselinePath := cfg.ResolveBaselinePath(home)
	out, err := runBaselineCreate(cfg, home)
	if err != nil {
		t.Fatalf("baseline create error: %v\noutput: %s", err, out)
	}
	// baseline.json 应存在
	if _, err := os.ReadFile(baselinePath); err != nil {
		t.Fatalf("baseline.json 应存在: %v", err)
	}
	// 应含指纹(baseline.wildcard-bash 命中 Bash(*))
	bs, err := suppression.LoadBaseline(baselinePath)
	if err != nil {
		t.Fatalf("加载 baseline 失败: %v", err)
	}
	if bs == nil || len(bs.Fingerprints) == 0 {
		t.Fatalf("baseline 应含至少一条指纹, got %+v", bs)
	}
}

// TestBaselinePrune 验证 sentinel baseline --prune 删除已不复现的指纹。
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
	baselinePath := cfg.ResolveBaselinePath(home)

	// (1) 先 create 生成 baseline
	if _, err := runBaselineCreate(cfg, home); err != nil {
		t.Fatalf("baseline create error: %v", err)
	}
	bs, err := suppression.LoadBaseline(baselinePath)
	if err != nil {
		t.Fatalf("加载 baseline 失败: %v", err)
	}
	if len(bs.Fingerprints) == 0 {
		t.Fatal("baseline create 应产出指纹")
	}
	fpCount := len(bs.Fingerprints)

	// (2) 往 baseline 塞一条假指纹(模拟已不复现的旧 finding)
	for k := range bs.Fingerprints {
		bs.Fingerprints["fake-stale-fingerprint-"+k] = true
		break
	}
	if len(bs.Fingerprints) != fpCount+1 {
		t.Fatalf("塞假指纹后应有 %d 条, got %d", fpCount+1, len(bs.Fingerprints))
	}
	if err := bs.Save(baselinePath); err != nil {
		t.Fatal(err)
	}

	// (3) prune:应删掉假指纹,保留真实指纹
	if _, err := runBaselinePrune(cfg, home); err != nil {
		t.Fatalf("baseline prune error: %v", err)
	}
	bs2, err := suppression.LoadBaseline(baselinePath)
	if err != nil {
		t.Fatalf("加载 pruned baseline 失败: %v", err)
	}
	if len(bs2.Fingerprints) != fpCount {
		t.Fatalf("prune 后应剩 %d 条指纹(=复现的), got %d", fpCount, len(bs2.Fingerprints))
	}
	for k := range bs2.Fingerprints {
		if strings.HasPrefix(k, "fake-stale-fingerprint-") {
			t.Fatalf("prune 应删除假指纹,但仍存在: %s", k)
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
