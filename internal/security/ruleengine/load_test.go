package ruleengine

import (
	"os"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

// ── brief 给定的 3 个测试 ──

func TestMergeOverride(t *testing.T) {
	builtin := []Rule{{ID: "x", Severity: "high"}}
	user := []Rule{{ID: "x", Severity: "low"}} // 覆盖
	merged := Merge(builtin, user)
	if len(merged) != 1 || merged[0].Severity != "low" {
		t.Fatal("user should override builtin")
	}
}

func TestMergeDisableByMissingMatch(t *testing.T) {
	builtin := []Rule{{ID: "x", Severity: "high", AssetType: "settings",
		Match: MatchNode{raw: map[string]any{"field": "model", "op": "eq", "value": "x"}}}}
	user := []Rule{{ID: "x", Severity: "high", AssetType: "settings"}} // 无 match = 禁用
	merged := Merge(builtin, user)
	if len(merged) != 1 {
		t.Fatal("want 1 merged")
	}
	// 禁用规则进队列但 Eval 永不命中(已在 Task 4 验证)
	if len(merged[0].Match.raw) != 0 {
		t.Fatal("merged rule should have empty match (disabled)")
	}
}

func TestLoadDirMissingIsEmpty(t *testing.T) {
	rules, errs := LoadDir("/nonexistent/path", "global")
	if len(rules) != 0 || len(errs) != 0 {
		t.Fatal("missing dir should be empty, no error")
	}
}

// ── 补充测试 ──

func TestMergePreservesOrder(t *testing.T) {
	layer1 := []Rule{
		{ID: "a", Severity: "high"},
		{ID: "b", Severity: "high"},
	}
	layer2 := []Rule{
		{ID: "c", Severity: "low"},
		{ID: "a", Severity: "low"}, // 覆盖 a,保持原位
	}
	merged := Merge(layer1, layer2)
	if len(merged) != 3 {
		t.Fatalf("want 3 merged, got %d", len(merged))
	}
	if merged[0].ID != "a" || merged[0].Severity != "low" {
		t.Errorf("a should be overridden in place: got id=%s sev=%s", merged[0].ID, merged[0].Severity)
	}
	if merged[1].ID != "b" {
		t.Errorf("b should stay at index 1: got %s", merged[1].ID)
	}
	if merged[2].ID != "c" {
		t.Errorf("c should be appended at index 2: got %s", merged[2].ID)
	}
}

func TestLoadBuiltinReturnsRules(t *testing.T) {
	rules, errs := LoadBuiltin()
	if len(errs) != 0 {
		t.Fatalf("LoadBuiltin should have no errors, got %v", errs)
	}
	if len(rules) == 0 {
		t.Fatal("LoadBuiltin should return at least 1 rule")
	}
	for _, r := range rules {
		if r.Source != "builtin" {
			t.Errorf("rule %q Source = %q, want %q", r.ID, r.Source, "builtin")
		}
	}
}

func TestLoadDirParsesYaml(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `rules:
  - id: test-001
    severity: high
    asset_type: settings
    match:
      field: model
      op: eq
      value: x
`
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	rules, errs := LoadDir(dir, "global")
	if len(errs) != 0 {
		t.Fatalf("LoadDir should have no errors, got %v", errs)
	}
	if len(rules) != 1 || rules[0].ID != "test-001" {
		t.Fatalf("want 1 rule test-001, got %d rules", len(rules))
	}
	if rules[0].Source == "" {
		t.Error("Source should be set")
	}
}

func TestLoadDirBadYamlReportsError(t *testing.T) {
	dir := t.TempDir()
	badYaml := `rules:
  - id: test-bad
    severity: high
    asset_type: settings
   bad indent
`
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(badYaml), 0644); err != nil {
		t.Fatal(err)
	}
	rules, errs := LoadDir(dir, "global")
	if len(errs) == 0 {
		t.Fatal("bad yaml should report error")
	}
	if len(rules) != 0 {
		t.Errorf("bad yaml should yield 0 rules, got %d", len(rules))
	}
}

func TestLoadForScanMergesBuiltinGlobalProject(t *testing.T) {
	home := t.TempDir()

	// 全局规则
	globalDir := filepath.Join(home, ".claude-sentinel", "rules")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatal(err)
	}
	globalYaml := `rules:
  - id: global-001
    severity: medium
    asset_type: settings
    match:
      field: model
      op: eq
      value: x
`
	if err := os.WriteFile(filepath.Join(globalDir, "global.yaml"), []byte(globalYaml), 0644); err != nil {
		t.Fatal(err)
	}

	// 项目规则
	projectDir := t.TempDir()
	projectRulesDir := filepath.Join(projectDir, ".sentinel", "rules")
	if err := os.MkdirAll(projectRulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	projectYaml := `rules:
  - id: project-001
    severity: high
    asset_type: permissions
    match:
      field: allow
      op: contains
      value: "Bash(rm:*)"
`
	if err := os.WriteFile(filepath.Join(projectRulesDir, "project.yaml"), []byte(projectYaml), 0644); err != nil {
		t.Fatal(err)
	}

	inv := &configengine.Inventory{
		Projects: []configengine.Project{{Path: projectDir, Name: "test-project"}},
	}

	rules, errs := LoadForScan(home, inv)
	if len(errs) != 0 {
		t.Fatalf("LoadForScan should have no errors, got %v", errs)
	}

	ids := make(map[string]bool)
	for _, r := range rules {
		ids[r.ID] = true
	}
	// 至少有内置规则
	hasBuiltin := false
	for _, r := range rules {
		if r.Source == "builtin" {
			hasBuiltin = true
			break
		}
	}
	if !hasBuiltin {
		t.Error("should have at least 1 builtin rule")
	}
	if !ids["global-001"] {
		t.Error("should have global-001 rule")
	}
	if !ids["project-001"] {
		t.Error("should have project-001 rule")
	}
}

func TestLoadForScanProjectIsolation(t *testing.T) {
	home := t.TempDir()

	projectDir := t.TempDir()
	projectRulesDir := filepath.Join(projectDir, ".sentinel", "rules")
	if err := os.MkdirAll(projectRulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	projectYaml := `rules:
  - id: project-isolated-001
    severity: high
    asset_type: permissions
    match:
      field: allow
      op: contains
      value: "Bash(curl:*)"
`
	if err := os.WriteFile(filepath.Join(projectRulesDir, "rules.yaml"), []byte(projectYaml), 0644); err != nil {
		t.Fatal(err)
	}

	inv := &configengine.Inventory{
		Projects: []configengine.Project{{Path: projectDir, Name: "test-project"}},
	}

	rules, errs := LoadForScan(home, inv)
	if len(errs) != 0 {
		t.Fatalf("LoadForScan should have no errors, got %v", errs)
	}

	// 项目规则应带 projectPath 标记
	var projectRule *Rule
	for i := range rules {
		if rules[i].ID == "project-isolated-001" {
			projectRule = &rules[i]
			break
		}
	}
	if projectRule == nil {
		t.Fatal("project-isolated-001 not found in merged rules")
	}
	if projectRule.ProjectPath != projectDir {
		t.Errorf("ProjectPath = %q, want %q", projectRule.ProjectPath, projectDir)
	}

	// builtin 和 global 规则不应有 projectPath
	for i := range rules {
		if rules[i].ID != "project-isolated-001" && rules[i].ProjectPath != "" {
			t.Errorf("rule %q has ProjectPath %q, want empty", rules[i].ID, rules[i].ProjectPath)
		}
	}
}

func TestLoadForScanOverrideBuiltinByGlobal(t *testing.T) {
	home := t.TempDir()
	globalDir := filepath.Join(home, ".claude-sentinel", "rules")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 获取内置规则 ID,用全局规则覆盖它
	builtin, _ := LoadBuiltin()
	if len(builtin) == 0 {
		t.Fatal("builtin rules empty")
	}
	builtinID := builtin[0].ID
	overrideYaml := "rules:\n  - id: " + builtinID + "\n    severity: low\n    asset_type: settings\n    description: overridden\n"
	if err := os.WriteFile(filepath.Join(globalDir, "override.yaml"), []byte(overrideYaml), 0644); err != nil {
		t.Fatal(err)
	}

	inv := &configengine.Inventory{}
	rules, errs := LoadForScan(home, inv)
	if len(errs) != 0 {
		t.Fatalf("LoadForScan should have no errors, got %v", errs)
	}

	for _, r := range rules {
		if r.ID == builtinID {
			if r.Severity != "low" {
				t.Errorf("overridden rule severity = %q, want %q", r.Severity, "low")
			}
			return
		}
	}
	t.Fatal("overridden rule not found in merged result")
}

func TestLoadForScanNoInventoryProjects(t *testing.T) {
	// 无项目的 inventory,只加载 builtin(无全局规则目录)
	home := t.TempDir()
	inv := &configengine.Inventory{}
	rules, errs := LoadForScan(home, inv)
	if len(errs) != 0 {
		t.Fatalf("LoadForScan should have no errors, got %v", errs)
	}
	if len(rules) == 0 {
		t.Fatal("should have at least builtin rules")
	}
}

func TestLoadDirNonDirectoryErrors(t *testing.T) {
	// 路径存在但不是目录(如用户误把 rules 建成普通文件)→ 报错而非静默返回空
	dir := t.TempDir()
	filePath := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	rules, errs := LoadDir(filePath, "global")
	if len(errs) != 1 {
		t.Fatalf("non-directory path should report 1 error, got %d: %v", len(errs), errs)
	}
	if len(rules) != 0 {
		t.Errorf("non-directory path should yield 0 rules, got %d", len(rules))
	}
}

func TestLoadForScanMultiProjectSameIDCoexists(t *testing.T) {
	home := t.TempDir()

	mkProject := func(dir, ruleValue string) {
		rdir := filepath.Join(dir, ".sentinel", "rules")
		if err := os.MkdirAll(rdir, 0755); err != nil {
			t.Fatal(err)
		}
		yaml := "rules:\n  - id: shared-rule\n    severity: high\n    asset_type: permissions\n    match:\n      field: allow\n      op: contains\n      value: \"" + ruleValue + "\"\n"
		if err := os.WriteFile(filepath.Join(rdir, "r.yaml"), []byte(yaml), 0644); err != nil {
			t.Fatal(err)
		}
	}

	projA := t.TempDir()
	projB := t.TempDir()
	mkProject(projA, "Bash(rm:*)")
	mkProject(projB, "Bash(curl:*)")

	inv := &configengine.Inventory{
		Projects: []configengine.Project{{Path: projA, Name: "a"}, {Path: projB, Name: "b"}},
	}
	rules, errs := LoadForScan(home, inv)
	if len(errs) != 0 {
		t.Fatalf("LoadForScan should have no errors, got %v", errs)
	}

	// 两条 shared-rule 都应存活,分别带各自的 ProjectPath
	var aRule, bRule *Rule
	for i := range rules {
		if rules[i].ID != "shared-rule" {
			continue
		}
		switch rules[i].ProjectPath {
		case projA:
			aRule = &rules[i]
		case projB:
			bRule = &rules[i]
		}
	}
	if aRule == nil || bRule == nil {
		t.Fatalf("both projects' shared-rule must survive; got aRule=%v bRule=%v", aRule != nil, bRule != nil)
	}
}
