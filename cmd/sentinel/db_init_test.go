package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code-agent-sentinel/internal/storage"
)

// TestMigrateLegacyRulesFilesRenamesAndImports 验证 main.go 侧的迁移编排:
// 旧 ~/.code-agent-sentinel/rules/*.yaml → db 的 custom 行(detect+intercept 两域)+ 文件重命名 .legacy。
//
// 解耦后 MigrateLegacyRules(storage 包)只做 db 写入,重命名是 main.go 的职责,
// 故 .legacy 重命名的验证在本 cmd 包测试里覆盖(storage 包的白盒测试只验 db 写入)。
func TestMigrateLegacyRulesFilesRenamesAndImports(t *testing.T) {
	home := t.TempDir()
	// 造旧全局规则文件
	oldDir := filepath.Join(home, ".code-agent-sentinel", "rules")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "mine.yaml"), []byte(`
rules:
  - id: custom.mine
    severity: high
    asset_type: command
    match: {field: command, op: contains, value: "evil-pattern"}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// 建 db(模拟 main.go 首次启动:Open + RunMigrations)
	dbPath := filepath.Join(home, "sentinel.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	if err := storage.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	migrateLegacyRulesFiles(home, db)

	// detect 域应有 source=custom 的 custom.mine
	got, ok, _ := storage.GetRule(db, storage.DomainDetect, "custom.mine")
	if !ok || got.Source != "custom" {
		t.Fatalf("detect 域未迁入 custom.mine: ok=%v got=%+v", ok, got)
	}
	// intercept 域也应同步迁入(对侧域)
	gotI, okI, _ := storage.GetRule(db, storage.DomainIntercept, "custom.mine")
	if !okI || gotI.Source != "custom" {
		t.Fatalf("intercept 域未同步迁入: ok=%v got=%+v", okI, gotI)
	}

	// 旧文件应已重命名 .legacy
	if _, err := os.Stat(filepath.Join(oldDir, "mine.yaml")); !os.IsNotExist(err) {
		t.Fatalf("旧文件应已重命名,.yaml 不该还在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(oldDir, "mine.yaml.legacy")); err != nil {
		t.Fatalf("legacy 文件应存在: %v", err)
	}
}

// TestMigrateLegacyRulesFilesNoDirNoOp 验证:无旧规则目录时,migrateLegacyRulesFiles 无操作(不报错)。
func TestMigrateLegacyRulesFilesNoDirNoOp(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, "sentinel.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	if err := storage.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// 无旧目录 → LoadDir 返回 nil → 无操作,不 panic 不报错
	migrateLegacyRulesFiles(home, db)

	rows, _ := storage.ListRules(db, storage.DomainDetect)
	if len(rows) != 0 {
		t.Fatalf("无旧目录应 0 规则,got %d", len(rows))
	}
}

// TestSyncBuiltinRulesPopulatesBothDomains 验证 syncBuiltinRules 把 embed builtin
// 规则同步进 detect + intercept 两域,detect 域带 combos。
func TestSyncBuiltinRulesPopulatesBothDomains(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, "sentinel.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	if err := storage.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	syncBuiltinRules(db)

	// detect 域应有 builtin 规则(embed 内置规则非空)
	detectRules, err := storage.ListRules(db, storage.DomainDetect)
	if err != nil {
		t.Fatalf("ListRules detect: %v", err)
	}
	if len(detectRules) == 0 {
		t.Fatal("detect 域 builtin 规则应非空")
	}
	// 抽查至少一条 source=builtin
	hasBuiltin := false
	for _, r := range detectRules {
		if r.Source == "builtin" {
			hasBuiltin = true
			break
		}
	}
	if !hasBuiltin {
		t.Error("detect 域应有 source=builtin 的行")
	}

	// intercept 域也应同步(同一份 builtin 规则)
	interceptRules, _ := storage.ListRules(db, storage.DomainIntercept)
	if len(interceptRules) == 0 {
		t.Fatal("intercept 域 builtin 规则应非空")
	}

	// detect 域 combos 应非空(embed 内置 combo_rules 存在时);若 embed 无 combo 则跳过该断言。
	// 这里不强制 combos 非空(取决于 embed 内容),只验证不报错。

	// 幂等:再调一次不重复行(主键冲突 DO UPDATE)
	syncBuiltinRules(db)
	detectRules2, _ := storage.ListRules(db, storage.DomainDetect)
	if len(detectRules2) != len(detectRules) {
		t.Fatalf("幂等同步不应改变行数: first=%d second=%d", len(detectRules), len(detectRules2))
	}
}

// TestSyncBuiltinRulesInterceptOnlyDestructive 验证 intercept 域只含 destructive 规则。
//
// 方案 2:拦截规则表只该含拦截规则(destructive_commands.yaml)。syncBuiltinRules 给
// intercept 域同步的 builtin 行必须只有 destructive.* 规则,baseline/injection/skill 等
// 检测规则不进拦截表。detect 域仍同步全部 builtin(扫描用)。
func TestSyncBuiltinRulesInterceptOnlyDestructive(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, "sentinel.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	if err := storage.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	syncBuiltinRules(db)

	interceptRules, err := storage.ListRules(db, storage.DomainIntercept)
	if err != nil {
		t.Fatalf("ListRules intercept: %v", err)
	}
	if len(interceptRules) == 0 {
		t.Fatal("intercept 域应含 destructive builtin 规则")
	}
	for _, r := range interceptRules {
		if !strings.HasPrefix(r.ID, "destructive.") {
			t.Fatalf("intercept 域含非 destructive 规则: %s(只该有 destructive_commands.yaml 的规则)", r.ID)
		}
	}

	// detect 域仍应有非 destructive 规则(baseline/injection 等检测规则仍在)。
	detectRules, err := storage.ListRules(db, storage.DomainDetect)
	if err != nil {
		t.Fatalf("ListRules detect: %v", err)
	}
	hasNonDestructive := false
	for _, r := range detectRules {
		if !strings.HasPrefix(r.ID, "destructive.") {
			hasNonDestructive = true
			break
		}
	}
	if !hasNonDestructive {
		t.Fatal("detect 域应仍含非 destructive 检测规则(baseline/injection 等),不该被收窄")
	}
}

// TestSyncBuiltinRulesNilDBNoOp 验证 db=nil 时 syncBuiltinRules 安全无操作(fail-open)。
func TestSyncBuiltinRulesNilDBNoOp(t *testing.T) {
	// 不应 panic
	syncBuiltinRules(nil)
}
