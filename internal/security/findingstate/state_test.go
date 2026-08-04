package findingstate

import (
	"os"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/storage"
)

func newTestStates(t *testing.T) *States {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewStates(db)
}

func TestNewStatesEmpty(t *testing.T) {
	s := newTestStates(t)
	if s == nil {
		t.Fatal("NewStates 应非 nil")
	}
	if s.Items == nil {
		t.Fatal("Items 应初始化")
	}
	if len(s.Items) != 0 {
		t.Fatalf("空 db 应有空 Items: got %d", len(s.Items))
	}
}

func TestNewStatesSetAndMatch(t *testing.T) {
	s := newTestStates(t)
	s.Set("abc", State{Status: StatusAccepted, Priority: "P1", Note: "test", Source: SourceManual, UpdatedAt: "t1"})
	got, ok := s.Match("abc")
	if !ok {
		t.Fatal("Set 后 Match 应命中")
	}
	if got.Status != StatusAccepted || got.Priority != "P1" || got.Note != "test" {
		t.Errorf("字段不匹配: %+v", got)
	}
}

func TestNewStatesSetPersisted(t *testing.T) {
	s := newTestStates(t)
	s.Set("abc", State{Status: StatusAccepted, Source: SourceManual, UpdatedAt: "t1"})

	// 重新从 db 加载,验证持久化
	s2 := NewStates(s.db)
	got, ok := s2.Match("abc")
	if !ok {
		t.Fatal("reload 后 Match 应命中")
	}
	if got.Status != StatusAccepted {
		t.Errorf("Status 不匹配: %+v", got)
	}
}

func TestNewStatesRemove(t *testing.T) {
	s := newTestStates(t)
	s.Set("abc", State{Status: StatusAccepted, Source: SourceManual, UpdatedAt: "t1"})
	if !s.Remove("abc") {
		t.Error("Remove 应返回 true")
	}
	if _, ok := s.Match("abc"); ok {
		t.Error("Remove 后 Match 应不命中")
	}
	// 重新加载确认 db 中也删了
	s2 := NewStates(s.db)
	if _, ok := s2.Match("abc"); ok {
		t.Error("reload 后 Match 应不命中(db 也应删了)")
	}
}

func TestNewStatesBulkAccept(t *testing.T) {
	s := newTestStates(t)
	s.Set("keep", State{Status: StatusResolved, Source: SourceManual, UpdatedAt: "old"})
	s.BulkAccept([]string{"a", "b", "keep"}, SourceBulkAccept, "now")
	// keep 已是 resolved,BulkAccept 不应覆盖已有非 open 状态
	if got, _ := s.Match("keep"); got.Status != StatusResolved {
		t.Errorf("keep overwritten: %+v", got)
	}
	// 新 fingerprint 写 accepted
	for _, fp := range []string{"a", "b"} {
		got, ok := s.Match(fp)
		if !ok || got.Status != StatusAccepted || got.Source != SourceBulkAccept || got.UpdatedAt != "now" {
			t.Errorf("%s = %+v ok=%v", fp, got, ok)
		}
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "finding_states.yaml")

	states := &States{Items: []State{
		{Fingerprint: "abc123", Status: StatusAccepted, Priority: "P2", Note: "重定向到 /tmp", Source: "manual", UpdatedAt: "2026-07-27T00:00:00Z"},
		{Fingerprint: "def456", Status: StatusResolved, Source: "manual", UpdatedAt: "2026-07-27T00:00:00Z"},
	}}
	if err := states.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 文件权限 0o600
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want 0o600", perm)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Items) != 2 {
		t.Fatalf("len = %d, want 2", len(loaded.Items))
	}
	if got := loaded.Items[0]; got.Fingerprint != "abc123" || got.Status != StatusAccepted || got.Priority != "P2" || got.Note != "重定向到 /tmp" {
		t.Errorf("item0 = %+v", got)
	}
}

func TestLoadMissingFileReturnsNil(t *testing.T) {
	loaded, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if loaded != nil {
		t.Errorf("expected nil, got %+v", loaded)
	}
}

func TestSetUpserts(t *testing.T) {
	s := &States{}
	s.Set("abc", State{Status: StatusOpen, Priority: "P1", Source: SourceManual, UpdatedAt: "t1"})
	if got, ok := s.Match("abc"); !ok || got.Status != StatusOpen || got.Priority != "P1" {
		t.Fatalf("after first Set: %+v ok=%v", got, ok)
	}
	// 再次 Set 同 fingerprint → 更新(覆盖)
	s.Set("abc", State{Status: StatusResolved, Source: SourceManual, UpdatedAt: "t2"})
	if got, ok := s.Match("abc"); !ok || got.Status != StatusResolved {
		t.Fatalf("after upsert: %+v ok=%v", got, ok)
	}
	if len(s.Items) != 1 {
		t.Errorf("len = %d, want 1 (upsert not append)", len(s.Items))
	}
}

func TestBulkAccept(t *testing.T) {
	s := &States{Items: []State{
		{Fingerprint: "keep", Status: StatusResolved, UpdatedAt: "old"},
	}}
	s.BulkAccept([]string{"a", "b", "keep"}, SourceBulkAccept, "now")
	// keep 已是 resolved,BulkAccept 不应覆盖已有非 open 状态
	if got, _ := s.Match("keep"); got.Status != StatusResolved {
		t.Errorf("keep overwritten: %+v", got)
	}
	// 新 fingerprint 写 accepted
	for _, fp := range []string{"a", "b"} {
		got, ok := s.Match(fp)
		if !ok || got.Status != StatusAccepted || got.Source != SourceBulkAccept || got.UpdatedAt != "now" {
			t.Errorf("%s = %+v ok=%v", fp, got, ok)
		}
	}
}

func TestPruneReport(t *testing.T) {
	s := &States{Items: []State{
		{Fingerprint: "active1", Status: StatusResolved},
		{Fingerprint: "orphan1", Status: StatusAccepted},
		{Fingerprint: "orphan2", Status: StatusFalsePositive},
	}}
	active := []string{"active1", "unrelated"}
	orphans := s.PruneReport(active)
	if len(orphans) != 2 {
		t.Fatalf("orphans = %d, want 2", len(orphans))
	}
	// 不删除原记录
	if len(s.Items) != 3 {
		t.Errorf("PruneReport mutated items: %d", len(s.Items))
	}
}

func TestRemove(t *testing.T) {
	s := &States{Items: []State{{Fingerprint: "x", Status: StatusAccepted}}}
	if !s.Remove("x") {
		t.Error("Remove returned false")
	}
	if _, ok := s.Match("x"); ok {
		t.Error("still matched after Remove")
	}
	if s.Remove("x") {
		t.Error("Remove returned true for missing")
	}
}
