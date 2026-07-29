package storage

import (
	"testing"
)

// Task 10: MigrateLegacyRules 的白盒测试(解耦版)。
//
// 解耦设计:本函数只做 db 写入(纯 storage 职责,不依赖 ruleengine)。
// 调用方(main.go 的 migrateLegacyRulesFiles)负责 LoadDir 旧文件 +
// RuleToStoredRule 转换 + 重命名 .legacy。故本测试手工构造 []StoredRule
// 传入(不 import ruleengine,保持 storage 包纯净)。
//
// brief 最初的耦合版测试(MigrateLegacyRules(db, home, domain) 自己读旧文件)
// 已废弃;这里覆盖三个等价关注点:
//  1. 把 rules 作为 custom 行写入 detect + intercept 两域(source=custom)。
//  2. 幂等:同批 rules 再调一次不重复(UpsertRule 是 ON CONFLICT DO UPDATE)。
//  3. 空 rules 输入 → Imported=0,不报错。

// migStored 构造一条用于迁移测试的 StoredRule(source 字段由 MigrateLegacyRules
// 强制写 "custom",此处 Source 仅作占位;真正落库 source 由 UpsertRule 参数决定)。
func migStored(id, match string) StoredRule {
	return StoredRule{
		ID:        id,
		Severity:  "high",
		AssetType: "command",
		MatchJSON: match,
	}
}

// TestMigrateLegacyRulesImportsCustomBothDomains 验证:传入 []StoredRule 后,
// MigrateLegacyRules 把它们作为 source=custom 写入传入域(detect)和对侧域(intercept)。
func TestMigrateLegacyRulesImportsCustomBothDomains(t *testing.T) {
	db := newTestDB(t)
	rules := []StoredRule{migStored("custom.mine", `{"field":"command","op":"contains","value":"evil-pattern"}`)}

	rep, err := MigrateLegacyRules(db, DomainDetect, rules)
	if err != nil {
		t.Fatalf("MigrateLegacyRules: %v", err)
	}
	if rep.Imported != 1 {
		t.Fatalf("Imported = %d, want 1", rep.Imported)
	}

	// detect 域应有 source=custom 的 custom.mine
	got, ok, gErr := GetRule(db, DomainDetect, "custom.mine")
	if gErr != nil || !ok {
		t.Fatalf("detect 域未迁入 custom.mine: ok=%v err=%v", ok, gErr)
	}
	if got.Source != "custom" {
		t.Fatalf("detect 域 source = %q, want custom", got.Source)
	}
	// builtin_version 应为 NULL(custom 行)
	if got.BuiltinVersion != "" {
		t.Fatalf("custom 行 builtin_version 应空(NULL),got %q", got.BuiltinVersion)
	}

	// intercept 域也应同步迁入(对侧域)
	gotI, okI, _ := GetRule(db, DomainIntercept, "custom.mine")
	if !okI || gotI.Source != "custom" {
		t.Fatalf("intercept 域未同步迁入: ok=%v source=%q", okI, gotI.Source)
	}
}

// TestMigrateLegacyRulesIdempotentNoDuplicate 验证:同一批 rules 调用两次不产生重复行
// (UpsertRule 主键冲突走 DO UPDATE,Imported 仍计 1 但表内不重复)。
func TestMigrateLegacyRulesIdempotentNoDuplicate(t *testing.T) {
	db := newTestDB(t)
	rules := []StoredRule{migStored("custom.a", `{"field":"command","op":"contains","value":"x"}`)}

	if _, err := MigrateLegacyRules(db, DomainDetect, rules); err != nil {
		t.Fatalf("第一次 MigrateLegacyRules: %v", err)
	}
	// 第二次:同 rules 再调 —— UpsertRule 覆盖,不新增行
	rep, err := MigrateLegacyRules(db, DomainDetect, rules)
	if err != nil {
		t.Fatalf("第二次 MigrateLegacyRules: %v", err)
	}
	if rep.Imported != 1 {
		t.Fatalf("第二次 Imported = %d, want 1(每次都计 Imported,但表内不重复)", rep.Imported)
	}

	rows, err := ListRules(db, DomainDetect)
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("detect 域应有 1 行(不重复),got %d", len(rows))
	}
	rowsI, _ := ListRules(db, DomainIntercept)
	if len(rowsI) != 1 {
		t.Fatalf("intercept 域应有 1 行(不重复),got %d", len(rowsI))
	}
}

// TestMigrateLegacyRulesEmptyInput 验证:空 rules 输入 → Imported=0,不报错,两域无行。
func TestMigrateLegacyRulesEmptyInput(t *testing.T) {
	db := newTestDB(t)
	rep, err := MigrateLegacyRules(db, DomainDetect, nil)
	if err != nil {
		t.Fatalf("空输入 MigrateLegacyRules: %v", err)
	}
	if rep.Imported != 0 {
		t.Fatalf("空输入 Imported = %d, want 0", rep.Imported)
	}
	rows, _ := ListRules(db, DomainDetect)
	if len(rows) != 0 {
		t.Fatalf("空输入 detect 域应 0 行,got %d", len(rows))
	}
	rowsI, _ := ListRules(db, DomainIntercept)
	if len(rowsI) != 0 {
		t.Fatalf("空输入 intercept 域应 0 行,got %d", len(rowsI))
	}
}

// TestMigrateLegacyRulesInterceptDomainFlipsOther 验证:以 intercept 域调用时,
// 对侧域翻转为 detect(确保 otherDomain 逻辑双向正确,不止 detect→intercept 单向)。
func TestMigrateLegacyRulesInterceptDomainFlipsOther(t *testing.T) {
	db := newTestDB(t)
	rules := []StoredRule{migStored("custom.b", `{"value":"y"}`)}

	rep, err := MigrateLegacyRules(db, DomainIntercept, rules)
	if err != nil {
		t.Fatalf("MigrateLegacyRules(intercept): %v", err)
	}
	if rep.Imported != 1 {
		t.Fatalf("Imported = %d, want 1", rep.Imported)
	}
	// intercept 域(传入域)
	gotI, okI, _ := GetRule(db, DomainIntercept, "custom.b")
	if !okI || gotI.Source != "custom" {
		t.Fatalf("intercept 域未迁入: ok=%v source=%q", okI, gotI.Source)
	}
	// detect 域(对侧,翻转后)
	gotD, okD, _ := GetRule(db, DomainDetect, "custom.b")
	if !okD || gotD.Source != "custom" {
		t.Fatalf("detect 域(对侧)未同步迁入: ok=%v source=%q", okD, gotD.Source)
	}
}
