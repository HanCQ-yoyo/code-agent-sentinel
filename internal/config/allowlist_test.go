package config

import (
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/storage"
)

func newTestAllowlistStore(t *testing.T) *AllowlistStore {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewAllowlistStore(db)
}

func TestAllowlistLoadMissingFile(t *testing.T) {
	s := newTestAllowlistStore(t)
	list, err := s.Load()
	if err != nil {
		t.Fatalf("空 db 应 nil-safe: %v", err)
	}
	if list == nil {
		t.Fatal("空清单应返回非 nil 切片")
	}
	if len(list) != 0 {
		t.Fatalf("空清单应返回空切片: %v", list)
	}
}

func TestAllowlistSaveLoadRoundTrip(t *testing.T) {
	s := newTestAllowlistStore(t)
	want := []string{"rm -rf node_modules", "git clean -fdx dist"}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("round-trip 丢失: got %v want %v", got, want)
	}
	// DB 按 ORDER BY command 排序,故返回顺序为字母序而非插入序。
	// 用 map 验证元素完整性,不依赖顺序。
	gotMap := make(map[string]bool, len(got))
	for _, g := range got {
		gotMap[g] = true
	}
	for _, w := range want {
		if !gotMap[w] {
			t.Fatalf("round-trip 丢失 %q: got %v", w, got)
		}
	}
}

func TestAllowlistMatchesExact(t *testing.T) {
	s := newTestAllowlistStore(t)
	if err := s.Save([]string{"rm -rf node_modules"}); err != nil {
		t.Fatal(err)
	}
	if !s.Matches("rm -rf node_modules") {
		t.Fatal("精确匹配应命中")
	}
	if s.Matches("rm -rf node_modules extra") {
		t.Fatal("非精确不应命中")
	}
	if s.Matches("rm -rf /") {
		t.Fatal("未列命令不应命中")
	}
}

func TestAllowlistMatchesTrimWhitespace(t *testing.T) {
	s := newTestAllowlistStore(t)
	if err := s.Save([]string{"  rm -rf node_modules  "}); err != nil {
		t.Fatal(err)
	}
	if !s.Matches("rm -rf node_modules") {
		t.Fatal("trim 后应匹配")
	}
	if err := s.Save([]string{"git clean -fdx dist"}); err != nil {
		t.Fatal(err)
	}
	if !s.Matches("  git clean -fdx dist  ") {
		t.Fatal("查询带空白 trim 后应匹配")
	}
}

func TestAllowlistMatchesPanicFalse(t *testing.T) {
	s := newTestAllowlistStore(t)
	if err := s.Save([]string{"rm -rf node_modules"}); err != nil {
		t.Fatal(err)
	}
	if s.Matches("rm -rf /") {
		t.Fatal("未列命令不应命中")
	}
	if s.Matches("") {
		t.Fatal("空命令不应命中")
	}
	if s.Matches("   ") {
		t.Fatal("纯空白命令 trim 后为空,不应命中")
	}
}
