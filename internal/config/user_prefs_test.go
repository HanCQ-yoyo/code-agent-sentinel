package config

import (
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/storage"
)

func TestUserPrefsGetSet(t *testing.T) {
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
	s := NewUserPrefsStore(db)

	// 初始读取不存在 key 返回空串
	v, err := s.Get("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if v != "" {
		t.Errorf("expected empty string, got %q", v)
	}

	// 写入并读回
	if err := s.Set("language", "en"); err != nil {
		t.Fatal(err)
	}
	v, err = s.Get("language")
	if err != nil {
		t.Fatal(err)
	}
	if v != "en" {
		t.Errorf("expected 'en', got %q", v)
	}

	// 覆盖写入
	if err := s.Set("language", "zh"); err != nil {
		t.Fatal(err)
	}
	v, err = s.Get("language")
	if err != nil {
		t.Fatal(err)
	}
	if v != "zh" {
		t.Errorf("expected 'zh', got %q", v)
	}

	// JSON 值存取
	if err := s.Set("favorites", `["a","b"]`); err != nil {
		t.Fatal(err)
	}
	v, err = s.Get("favorites")
	if err != nil {
		t.Fatal(err)
	}
	if v != `["a","b"]` {
		t.Errorf("expected JSON array, got %q", v)
	}
}

func TestUserPrefsNilDB(t *testing.T) {
	s := NewUserPrefsStore(nil)
	v, err := s.Get("any")
	if err != nil {
		t.Fatal(err)
	}
	if v != "" {
		t.Errorf("nil db: expected empty string, got %q", v)
	}
	if err := s.Set("any", "val"); err != nil {
		t.Fatal(err)
	}
}

