// internal/history/store_test.go
package history

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security"
)

func TestSaveGetLatest(t *testing.T) {
	s := NewStore(t.TempDir())
	rec := ScanRecord{
		ID:        "2026-07-06-14-30-05-a1b2c3d4",
		StartedAt: time.Date(2026, 7, 6, 14, 30, 5, 0, time.UTC),
		Findings: []security.Finding{
			{DetectorID: "baseline", RuleID: "r1", Severity: security.SeverityHigh, AssetID: "a1"},
		},
		HealthScore: &security.HealthScore{Score: 80, Band: "良"},
		Inventory:   &configengine.Inventory{Assets: []configengine.Asset{{ID: "a1", Type: "settings", Name: "n"}}},
	}
	if err := s.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Get(rec.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != rec.ID || len(got.Findings) != 1 || got.HealthScore.Score != 80 {
		t.Fatalf("往返不一致: %+v", got)
	}
	latest, err := s.Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if latest.ID != rec.ID {
		t.Fatalf("Latest 应为刚存的记录,got %s", latest.ID)
	}
}

func TestListOrderAndEmpty(t *testing.T) {
	s := NewStore(t.TempDir())
	got, err := s.List()
	if err != nil {
		t.Fatalf("空目录 List: %v", err)
	}
	if got != nil {
		t.Fatalf("空目录应返回 nil,got %+v", got)
	}
	// 存 3 条,时间递增
	for i, ts := range []time.Time{
		time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 6, 11, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC),
	} {
		rec := ScanRecord{ID: fmt.Sprintf("rec-%d", i), StartedAt: ts}
		if err := s.Save(rec); err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
	}
	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("应有 3 条,got %d", len(list))
	}
	if list[0].ID != "rec-2" || list[2].ID != "rec-0" {
		t.Fatalf("倒序错误: %+v", list)
	}
}

func TestDeleteAndNotFound(t *testing.T) {
	s := NewStore(t.TempDir())
	rec := ScanRecord{ID: "del-me", StartedAt: time.Now().UTC()}
	if err := s.Save(rec); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(rec.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(rec.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删除后应 ErrNotFound,got %v", err)
	}
	if err := s.Delete("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删不存在应 ErrNotFound,got %v", err)
	}
	if _, err := s.Latest(); err != nil {
		t.Fatalf("空目录 Latest 应 (nil,nil),got %v", err)
	}
}

func TestSameSecondNoConflict(t *testing.T) {
	s := NewStore(t.TempDir())
	ts := time.Date(2026, 7, 6, 14, 30, 5, 0, time.UTC)
	for i := 0; i < 3; i++ {
		rec := ScanRecord{ID: fmt.Sprintf("2026-07-06-14-30-05-%08d", i), StartedAt: ts}
		if err := s.Save(rec); err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("同秒 3 条应全部保留,got %d", len(list))
	}
}
