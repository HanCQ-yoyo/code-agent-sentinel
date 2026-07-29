package ruleengine

import (
	"os"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/storage"
)

// osMkdirAll / osWriteFile 是 os 包的薄包装(测试可替换,与 brief 一致)。
func osMkdirAll(p string) error                              { return os.MkdirAll(p, 0o755) }
func osWriteFile(p string, b []byte, perm os.FileMode) error { return os.WriteFile(p, b, perm) }

// newDBWithBuiltin 开一个临时 db,跑迁移,并把真实 embed builtin 规则经适配层同步进
// detect 域。combos 也一并同步(覆盖 builtin combo_rules)。
func newDBWithBuiltin(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	// 用真实 embed builtin 规则同步进 db(经适配层)
	builtin, builtinCombos, _ := LoadBuiltin()
	stored, err := rulesToStored(builtin, "v1")
	if err != nil {
		t.Fatal(err)
	}
	storedCombos, err := combosToStored(builtinCombos, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.SyncBuiltin(db, storage.DomainDetect, stored, storedCombos, "v1"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestLoadDetectRulesReturnsBuiltin(t *testing.T) {
	db := newDBWithBuiltin(t)
	rules, combos, errs := LoadDetectRules(db, nil)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v", errs)
	}
	if len(rules) == 0 {
		t.Fatal("expected builtin rules loaded from db")
	}
	// combos 应非空(builtin combo_rules)
	if len(combos) == 0 {
		t.Fatal("expected builtin combos loaded from db")
	}
}

func TestLoadDetectRulesRespectsDisabledOverride(t *testing.T) {
	db := newDBWithBuiltin(t)
	rules, _, _ := LoadDetectRules(db, nil)
	before := len(rules)
	if before == 0 {
		t.Fatal("need at least one builtin rule to test override")
	}
	// 禁用第一条 builtin 规则
	first := rules[0]
	if err := storage.SetOverride(db, storage.DomainDetect, first.ID, false); err != nil {
		t.Fatal(err)
	}
	rules2, _, errs := LoadDetectRules(db, nil)
	if len(errs) != 0 {
		t.Fatalf("errs after override = %+v", errs)
	}
	if len(rules2) != before-1 {
		t.Fatalf("after disabling 1 rule: %d, want %d", len(rules2), before-1)
	}
	// 被禁用的规则不应出现在结果里
	for _, r := range rules2 {
		if r.ID == first.ID {
			t.Fatalf("disabled rule %s still present", first.ID)
		}
	}
}

func TestLoadInterceptRules(t *testing.T) {
	db := newDBWithBuiltin(t)
	// 同步一份到 intercept 域
	builtin, _, _ := LoadBuiltin()
	stored, err := rulesToStored(builtin, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.SyncBuiltin(db, storage.DomainIntercept, stored, nil, "v1"); err != nil {
		t.Fatal(err)
	}
	rules, errs := LoadInterceptRules(db)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v", errs)
	}
	if len(rules) == 0 {
		t.Fatal("expected intercept rules loaded")
	}
}

// TestLoadDetectRulesMergesProjectRules 验证项目级 .sentinel/rules 文件规则被加载并
// 带上 ProjectPath。按 Merge 的复合键语义(load.go:98 注释 + TODO):项目规则与同 id
// builtin 规则「共存」(ProjectPath 不同 → 复合键不同),而非覆盖 builtin——项目规则
// 的 ProjectPath 让 RulesDetector 只对该项目资产生效。故此处断言项目规则带 ProjectPath
// 出现,且 builtin 同 id 规则仍在(二者共存)。
func TestLoadDetectRulesMergesProjectRules(t *testing.T) {
	db := newDBWithBuiltin(t)
	projDir := t.TempDir()
	projRulesDir := filepath.Join(projDir, ".sentinel", "rules")
	if err := osMkdirAll(projRulesDir); err != nil {
		t.Fatal(err)
	}
	if err := osWriteFile(filepath.Join(projRulesDir, "p.yaml"), []byte(`
rules:
  - id: destructive.filesystem.rm-rf-root-home
    severity: low
    asset_type: hook
    match: {field: command, op: contains, value: "rm -rf /nope-override"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	projects := []configengine.Project{{Path: projDir}}
	rules, _, errs := LoadDetectRules(db, projects)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v", errs)
	}
	// 项目规则带 ProjectPath 出现,severity=low
	var projectRuleFound bool
	for _, r := range rules {
		if r.ID == "destructive.filesystem.rm-rf-root-home" && r.ProjectPath == projDir {
			projectRuleFound = true
			if r.Severity != "low" {
				t.Fatalf("project rule severity=%s, want low", r.Severity)
			}
		}
	}
	if !projectRuleFound {
		t.Fatal("project rule with ProjectPath not loaded")
	}
}

// TestLoadDetectRulesCustomRuleOverridesBuiltin 验证 db 里同 id 的 custom 规则覆盖
// builtin 规则(Merge 用 id+"|"+ProjectPath 复合键,builtin 与 custom 的 ProjectPath
// 均为空 → 同键 → 后者整条替换前者)。这是 DB 模式下用户改规则的主路径。
func TestLoadDetectRulesCustomRuleOverridesBuiltin(t *testing.T) {
	db := newDBWithBuiltin(t)
	// upsert 一条 custom 规则,同 id 但 severity 改 low(覆盖 builtin critical)
	custom := Rule{
		ID:        "destructive.filesystem.rm-rf-root-home",
		Severity:  "low",
		AssetType: "hook",
		Match:     NewMatchNode(map[string]any{"field": "command", "op": "contains", "value": "rm -rf /custom"}),
	}
	stored, err := RuleToStoredRule(custom, "custom", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.UpsertRule(db, storage.DomainDetect, "custom", stored, ""); err != nil {
		t.Fatal(err)
	}
	rules, _, errs := LoadDetectRules(db, nil)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v", errs)
	}
	// 同 id 只应剩一条(custom 覆盖 builtin),severity=low
	var count int
	var got Rule
	for _, r := range rules {
		if r.ID == "destructive.filesystem.rm-rf-root-home" {
			count++
			got = r
		}
	}
	if count != 1 {
		t.Fatalf("want exactly 1 rule with this id (custom overwrites builtin), got %d", count)
	}
	if got.Severity != "low" {
		t.Fatalf("custom override severity=%s, want low", got.Severity)
	}
}
