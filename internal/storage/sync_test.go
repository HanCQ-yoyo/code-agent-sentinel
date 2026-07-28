package storage

import (
	"testing"
)

func toStored(id, match string) StoredRule {
	return StoredRule{
		ID: id, Severity: "high", AssetType: "command",
		MatchJSON: match, Source: "builtin", BuiltinVersion: "v1",
	}
}

func TestSyncBuiltinInsertsNew(t *testing.T) {
	db := newTestDB(t)
	res, err := SyncBuiltin(db, DomainDetect, []StoredRule{toStored("b1", `{"field":"command","op":"contains","value":"a"}`)}, nil, "v1")
	if err != nil {
		t.Fatalf("SyncBuiltin: %v", err)
	}
	if res.Refreshed != 1 {
		t.Fatalf("Refreshed = %d, want 1", res.Refreshed)
	}
	got, ok, _ := GetRule(db, DomainDetect, "b1")
	if !ok || got.Source != "builtin" || got.BuiltinVersion != "v1" {
		t.Fatalf("GetRule = %+v", got)
	}
}

func TestSyncBuiltinOverwritesBuiltinKeepsCustom(t *testing.T) {
	db := newTestDB(t)
	// 先塞一条 builtin v1 + 一条 custom
	_ = UpsertRule(db, DomainDetect, "builtin", toStored("b1", `{"value":"old"}`), "v1")
	_ = UpsertRule(db, DomainDetect, "custom", toStored("c1", `{"value":"mine"}`), "")
	// 同步新版本:b1 内容变,c1 不动
	_, err := SyncBuiltin(db, DomainDetect, []StoredRule{toStored("b1", `{"value":"new"}`)}, nil, "v2")
	if err != nil {
		t.Fatal(err)
	}
	b1, _, _ := GetRule(db, DomainDetect, "b1")
	if b1.MatchJSON != `{"value":"new"}` || b1.BuiltinVersion != "v2" {
		t.Fatalf("b1 not refreshed: %+v", b1)
	}
	c1, _, _ := GetRule(db, DomainDetect, "c1")
	if c1.Source != "custom" {
		t.Fatalf("custom row should be untouched: %+v", c1)
	}
}

func TestSyncBuiltinKeepsOverrides(t *testing.T) {
	db := newTestDB(t)
	_ = UpsertRule(db, DomainDetect, "builtin", toStored("b1", `{"value":"old"}`), "v1")
	_ = SetOverride(db, DomainDetect, "b1", false) // 用户禁用 b1
	_, _ = SyncBuiltin(db, DomainDetect, []StoredRule{toStored("b1", `{"value":"new"}`)}, nil, "v2")
	enabled, exists, _ := GetOverride(db, DomainDetect, "b1")
	if !exists || enabled {
		t.Fatalf("override should survive sync: enabled=%v exists=%v", enabled, exists)
	}
}

func TestSyncBuiltinRemovesDeletedBuiltinReportsOrphan(t *testing.T) {
	db := newTestDB(t)
	// v1 有 b1,用户给 b1 设了 override;v2 删了 b1
	_ = UpsertRule(db, DomainDetect, "builtin", toStored("b1", `{"value":"x"}`), "v1")
	_ = SetOverride(db, DomainDetect, "b1", false)
	// SyncBuiltin v2:rules 列表里没有 b1 → 删 b1 的 builtin 行,override 成孤儿
	res, err := SyncBuiltin(db, DomainDetect, nil, nil, "v2")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.OrphanOverrides) != 1 || res.OrphanOverrides[0] != "b1" {
		t.Fatalf("orphans = %+v", res.OrphanOverrides)
	}
	// b1 行应已删
	_, ok, _ := GetRule(db, DomainDetect, "b1")
	if ok {
		t.Fatal("deleted builtin row should be gone")
	}
}

func TestSyncBuiltinIdempotent(t *testing.T) {
	db := newTestDB(t)
	rules := []StoredRule{toStored("b1", `{"value":"x"}`)}
	if _, err := SyncBuiltin(db, DomainDetect, rules, nil, "v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncBuiltin(db, DomainDetect, rules, nil, "v1"); err != nil {
		t.Fatal(err)
	}
	rows, _ := ListRules(db, DomainDetect)
	if len(rows) != 1 {
		t.Fatalf("idempotent sync should not duplicate: %d rows", len(rows))
	}
}
