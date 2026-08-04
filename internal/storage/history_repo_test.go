package storage

import "testing"

func sampleHistory(id, agentID, startedAt string) HistoryRow {
	return HistoryRow{
		ID: id, AgentID: agentID, StartedAt: startedAt,
		DurationNs:    5000000000,
		Scope:         "global",
		FindingCount:  3,
		HealthScore:   85,
		HealthBand:    "B",
		DetectorAvail: 3,
		DetectorTotal: 3,
		FindingsJSON:  []byte(`[{"rule_id":"r1"}]`),
		DetectorsJSON: []byte(`[{"name":"rules","available":true}]`),
		InventoryJSON: []byte(`{}`),
		ProjectsJSON:  []byte(`[]`),
	}
}

func TestHistoryInsertAndGet(t *testing.T) {
	db := newTestDB(t)
	r := sampleHistory("2026-08-04-10-00-00-a1b2c3d4", "claude-code", "2026-08-04T10:00:00Z")
	if err := InsertHistory(db, r); err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}
	got, ok, err := GetHistory(db, r.ID)
	if err != nil || !ok {
		t.Fatalf("GetHistory: ok=%v err=%v", ok, err)
	}
	if got.FindingCount != 3 || got.HealthScore != 85 {
		t.Fatalf("unexpected row: finding=%d health=%d", got.FindingCount, got.HealthScore)
	}
	if string(got.FindingsJSON) != `[{"rule_id":"r1"}]` {
		t.Fatalf("JSON roundtrip failed: %s", got.FindingsJSON)
	}
}

func TestHistoryMissing(t *testing.T) {
	db := newTestDB(t)
	_, ok, err := GetHistory(db, "nonexistent")
	if err != nil || ok {
		t.Fatalf("want ok=false nil err, got ok=%v err=%v", ok, err)
	}
}

func TestHistoryDelete(t *testing.T) {
	db := newTestDB(t)
	_ = InsertHistory(db, sampleHistory("id1", "claude-code", "2026-08-04T10:00:00Z"))
	if deleted, _ := DeleteHistory(db, "id1"); !deleted {
		t.Fatal("should be deleted")
	}
	if _, ok, _ := GetHistory(db, "id1"); ok {
		t.Fatal("should not exist after delete")
	}
}

func TestHistoryListSummaries(t *testing.T) {
	db := newTestDB(t)
	_ = InsertHistory(db, sampleHistory("id1", "claude-code", "2026-08-04T10:00:00Z"))
	_ = InsertHistory(db, sampleHistory("id2", "codex", "2026-08-04T11:00:00Z"))
	summaries, err := ListHistorySummaries(db)
	if err != nil {
		t.Fatalf("ListHistorySummaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("want 2 summaries, got %d", len(summaries))
	}
	// 倒序:最新在前
	if summaries[0].ID != "id2" || summaries[1].ID != "id1" {
		t.Fatalf("unexpected order: %+v", summaries)
	}
	// 验证摘要字段不含 JSON
	if summaries[0].FindingCount != 3 || summaries[0].AgentID != "codex" {
		t.Fatalf("summary fields wrong: %+v", summaries[0])
	}
}
