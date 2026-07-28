package storage

import (
	"path/filepath"
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func sampleRow(id string) StoredRule {
	return StoredRule{
		ID: id, Severity: "critical", AssetType: "command",
		MatchJSON: `{"field":"command","op":"contains","value":"rm -rf"}`,
		Dotall: false, Source: "custom",
	}
}

func TestUpsertAndListRules(t *testing.T) {
	db := newTestDB(t)
	if err := UpsertRule(db, DomainDetect, "custom", sampleRow("r1"), ""); err != nil {
		t.Fatalf("UpsertRule: %v", err)
	}
	rows, err := ListRules(db, DomainDetect)
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "r1" {
		t.Fatalf("ListRules = %+v", rows)
	}
}

func TestUpsertRuleOverwritesSameID(t *testing.T) {
	db := newTestDB(t)
	r := sampleRow("r1")
	_ = UpsertRule(db, DomainDetect, "custom", r, "")
	r.Severity = "high"
	_ = UpsertRule(db, DomainDetect, "custom", r, "")
	rows, _ := ListRules(db, DomainDetect)
	if len(rows) != 1 || rows[0].Severity != "high" {
		t.Fatalf("overwrite failed: %+v", rows)
	}
}

func TestGetRuleExists(t *testing.T) {
	db := newTestDB(t)
	_ = UpsertRule(db, DomainDetect, "builtin", sampleRow("b1"), "v1")
	got, ok, err := GetRule(db, DomainDetect, "b1")
	if err != nil || !ok {
		t.Fatalf("GetRule: ok=%v err=%v", ok, err)
	}
	if got.Source != "builtin" || got.BuiltinVersion != "v1" {
		t.Fatalf("GetRule = %+v", got)
	}
}

func TestGetRuleMissing(t *testing.T) {
	db := newTestDB(t)
	_, ok, err := GetRule(db, DomainDetect, "nope")
	if err != nil || ok {
		t.Fatalf("want ok=false nil err, got ok=%v err=%v", ok, err)
	}
}

func TestDeleteRuleCascadesOverride(t *testing.T) {
	db := newTestDB(t)
	_ = UpsertRule(db, DomainDetect, "custom", sampleRow("r1"), "")
	_ = SetOverride(db, DomainDetect, "r1", false)
	_ = DeleteRule(db, DomainDetect, "r1")
	_, ok, _ := GetRule(db, DomainDetect, "r1")
	if ok {
		t.Fatal("rule should be deleted")
	}
	// override 应被外键 CASCADE 清掉
	enabled, exists, _ := GetOverride(db, DomainDetect, "r1")
	if exists {
		t.Fatalf("override should be cascaded, got enabled=%v", enabled)
	}
}

func TestSetOverrideThenGet(t *testing.T) {
	db := newTestDB(t)
	_ = UpsertRule(db, DomainDetect, "builtin", sampleRow("b1"), "v1")
	_ = SetOverride(db, DomainDetect, "b1", false)
	enabled, exists, err := GetOverride(db, DomainDetect, "b1")
	if err != nil || !exists || enabled {
		t.Fatalf("GetOverride = enabled=%v exists=%v err=%v", enabled, exists, err)
	}
	// 翻回 true
	_ = SetOverride(db, DomainDetect, "b1", true)
	enabled, _, _ = GetOverride(db, DomainDetect, "b1")
	if !enabled {
		t.Fatal("override should be true after re-set")
	}
}

func TestListOrphanOverrides(t *testing.T) {
	db := newTestDB(t)
	_ = UpsertRule(db, DomainDetect, "builtin", sampleRow("b1"), "v1")
	_ = SetOverride(db, DomainDetect, "b1", false)
	// 直接删 rule 行(模拟 builtin 规则下版本被删),override 成孤儿
	_, _ = db.SQL().Exec("DELETE FROM detect_rules WHERE rule_id='b1'")
	orphans, err := ListOrphanOverrides(db, DomainDetect)
	if err != nil {
		t.Fatalf("ListOrphanOverrides: %v", err)
	}
	if len(orphans) != 1 || orphans[0] != "b1" {
		t.Fatalf("orphans = %+v", orphans)
	}
}

func TestDomainsAreIsolated(t *testing.T) {
	db := newTestDB(t)
	_ = UpsertRule(db, DomainDetect, "custom", sampleRow("r1"), "")
	_ = UpsertRule(db, DomainIntercept, "custom", sampleRow("r1"), "")
	// detect 与 intercept 是不同表,同 id 各自独立
	dRows, _ := ListRules(db, DomainDetect)
	iRows, _ := ListRules(db, DomainIntercept)
	if len(dRows) != 1 || len(iRows) != 1 {
		t.Fatalf("detect=%d intercept=%d", len(dRows), len(iRows))
	}
}
