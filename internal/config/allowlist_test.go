package config

import (
	"path/filepath"
	"testing"
)

func TestAllowlistLoadMissingFile(t *testing.T) {
	s := NewAllowlistStore(filepath.Join(t.TempDir(), "allowlist.yaml"))
	list, err := s.Load()
	if err != nil {
		t.Fatalf("文件不存在应 nil-safe: %v", err)
	}
	if list == nil {
		t.Fatal("空清单应返回非 nil 切片")
	}
	if len(list) != 0 {
		t.Fatalf("空清单应返回空切片: %v", list)
	}
}

func TestAllowlistSaveLoadRoundTrip(t *testing.T) {
	s := NewAllowlistStore(filepath.Join(t.TempDir(), "allowlist.yaml"))
	want := []string{"rm -rf node_modules", "git clean -fdx dist"}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("round-trip 丢失: got %v want %v", got, want)
	}
}

func TestAllowlistMatchesExact(t *testing.T) {
	s := NewAllowlistStore(filepath.Join(t.TempDir(), "allowlist.yaml"))
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
	s := NewAllowlistStore(filepath.Join(t.TempDir(), "allowlist.yaml"))
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
	s := NewAllowlistStore(filepath.Join(t.TempDir(), "allowlist.yaml"))
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
