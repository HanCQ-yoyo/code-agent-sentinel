package storage

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesFileAndWAL(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// WAL 模式应已开启(pragma 查询返回 wal)
	var mode string
	if err := db.sqlDB.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}

func TestSchemaInitializedFalseBeforeMigration(t *testing.T) {
	dir := t.TempDir()
	db, _ := Open(filepath.Join(dir, "test.db"))
	defer db.Close()

	got, err := SchemaInitialized(db)
	if err != nil {
		t.Fatalf("SchemaInitialized: %v", err)
	}
	if got {
		t.Fatal("fresh db should report schema not initialized")
	}
}

func TestRunMigrationsCreatesTables(t *testing.T) {
	dir := t.TempDir()
	db, _ := Open(filepath.Join(dir, "test.db"))
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	initialized, _ := SchemaInitialized(db)
	if !initialized {
		t.Fatal("after migration schema should be initialized")
	}
	// 四张 rules 表 + 四张 overrides 表 + 两张 combos 表都应存在
	for _, tbl := range []string{"detect_rules", "detect_overrides", "detect_combos",
		"intercept_rules", "intercept_overrides", "intercept_combos"} {
		var name string
		err := db.sqlDB.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing: %v", tbl, err)
		}
	}
}
