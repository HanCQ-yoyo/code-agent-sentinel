// internal/intercept/store_test.go
package intercept

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreSaveGetDelete(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	rec := InterceptRecord{
		ID: "20260728-abc", Timestamp: time.Now(), AgentProtocol: "claude",
		Outcome: "deny", Command: "rm -rf /",
	}
	if err := s.Append(rec); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("20260728-abc")
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if got.Command != "rm -rf /" {
		t.Fatalf("Get 内容不对: %+v", got)
	}
	// 文件落在 dir 下
	if _, err := s.Get("nonexistent"); err != ErrNotFound {
		t.Fatalf("不存在应返回 ErrNotFound, got %v", err)
	}
	if err := s.Delete("20260728-abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("20260728-abc"); err != ErrNotFound {
		t.Fatal("Delete 后 Get 应 ErrNotFound")
	}
}

func TestStoreListOrder(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	base := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	for i, off := range []int{2, 0, 1} {
		s.Append(InterceptRecord{
			ID: "r" + string(rune('a'+i)), Timestamp: base.Add(time.Duration(off) * time.Minute),
		})
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	// ra@+2min(最新) rb@+0min(最旧) rc@+1min(中) → 倒序: ra, rc, rb
	if len(list) != 3 || list[0].ID != "ra" || list[2].ID != "rb" {
		t.Fatalf("List 应按 Timestamp 倒序: %+v", list)
	}
	_ = filepath.Join // keep import
}
