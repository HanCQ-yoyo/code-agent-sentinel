package config

import (
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/storage"
)

func TestScheduleRepoCRUD(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := storage.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	r := NewScheduleRepo(db)

	// 初始空列表
	list, err := r.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty, got %d items", len(list))
	}

	// 插入一条
	if err := r.Upsert("claude-code", true, "30m"); err != nil {
		t.Fatal(err)
	}
	list, err = r.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}
	if list[0].AgentID != "claude-code" || !list[0].Enabled || list[0].Interval != "30m" {
		t.Errorf("unexpected row: %+v", list[0])
	}

	// 更新
	if err := r.Upsert("claude-code", false, "1h"); err != nil {
		t.Fatal(err)
	}
	list, err = r.List()
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Enabled || list[0].Interval != "1h" {
		t.Errorf("expected updated: %+v", list[0])
	}

	// 插入第二条
	if err := r.Upsert("codex", true, "15m"); err != nil {
		t.Fatal(err)
	}
	list, err = r.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}

	// 删除
	if err := r.Delete("claude-code"); err != nil {
		t.Fatal(err)
	}
	list, err = r.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].AgentID != "codex" {
		t.Errorf("expected [codex], got %+v", list)
	}
}

func TestScheduleRepoNilDB(t *testing.T) {
	r := NewScheduleRepo(nil)
	list, err := r.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("nil db: expected empty, got %d", len(list))
	}
	if err := r.Upsert("x", true, "30m"); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete("x"); err != nil {
		t.Fatal(err)
	}
}
