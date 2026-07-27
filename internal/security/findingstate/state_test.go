package findingstate

import (
	"os"
	"path/filepath"
	"testing"
)

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
