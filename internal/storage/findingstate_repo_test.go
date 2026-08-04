package storage

import "testing"

func TestFindingStateUpsertAndGet(t *testing.T) {
	db := newTestDB(t)
	if err := UpsertFindingState(db, "fp1", "accepted", "P2", "已知风险", "manual", "2026-08-04T00:00:00Z"); err != nil {
		t.Fatalf("UpsertFindingState: %v", err)
	}
	row, ok, err := GetFindingState(db, "fp1")
	if err != nil || !ok {
		t.Fatalf("GetFindingState: ok=%v err=%v", ok, err)
	}
	if row.Status != "accepted" || row.Priority != "P2" || row.Note != "已知风险" {
		t.Fatalf("unexpected row: %+v", row)
	}
}

func TestFindingStateUpsertOverwrites(t *testing.T) {
	db := newTestDB(t)
	_ = UpsertFindingState(db, "fp1", "open", "", "", "manual", "t1")
	_ = UpsertFindingState(db, "fp1", "resolved", "P1", "fixed", "manual", "t2")
	row, _, _ := GetFindingState(db, "fp1")
	if row.Status != "resolved" || row.Note != "fixed" {
		t.Fatalf("upsert should overwrite: %+v", row)
	}
}

func TestFindingStateMissing(t *testing.T) {
	db := newTestDB(t)
	_, ok, err := GetFindingState(db, "nonexistent")
	if err != nil || ok {
		t.Fatalf("want ok=false nil err, got ok=%v err=%v", ok, err)
	}
}

func TestFindingStateDelete(t *testing.T) {
	db := newTestDB(t)
	_ = UpsertFindingState(db, "fp1", "open", "", "", "manual", "t1")
	deleted, err := DeleteFindingState(db, "fp1")
	if err != nil || !deleted {
		t.Fatalf("DeleteFindingState: deleted=%v err=%v", deleted, err)
	}
	_, ok, _ := GetFindingState(db, "fp1")
	if ok {
		t.Fatal("should be deleted")
	}
	// 再删不存在的:deleted=false
	deleted2, _ := DeleteFindingState(db, "fp1")
	if deleted2 {
		t.Fatal("second delete should return false")
	}
}

func TestFindingStateList(t *testing.T) {
	db := newTestDB(t)
	_ = UpsertFindingState(db, "fp2", "resolved", "", "", "manual", "t2")
	_ = UpsertFindingState(db, "fp1", "accepted", "P2", "", "manual", "t1")
	rows, err := ListFindingStates(db)
	if err != nil {
		t.Fatalf("ListFindingStates: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	// 验证 alphabetically sorted by fingerprint
	if rows[0].Fingerprint != "fp1" || rows[1].Fingerprint != "fp2" {
		t.Fatalf("unexpected order: %+v", rows)
	}
}
