package storage

import "testing"

func sampleIntercept(id string) InterceptRow {
	return InterceptRow{
		ID: id, Timestamp: "2026-08-04T10:00:00Z", AgentProtocol: "claude",
		WorkingDir: "/home/user", Command: "rm -rf /tmp/test", Outcome: "deny",
		RuleID: "builtin.rm-rf", Severity: "critical", Reason: "destructive command",
		EvalDurationUS: 1500, SessionID: "sess-1", ToolName: "Bash",
		Confidence: "high", MatchedSpan: "rm -rf /tmp/test",
	}
}

func TestInterceptInsertAndGet(t *testing.T) {
	db := newTestDB(t)
	r := sampleIntercept("evt-1")
	if err := InsertIntercept(db, r); err != nil {
		t.Fatalf("InsertIntercept: %v", err)
	}
	got, ok, err := GetIntercept(db, "evt-1")
	if err != nil || !ok {
		t.Fatalf("GetIntercept: ok=%v err=%v", ok, err)
	}
	if got.Command != r.Command || got.Outcome != "deny" {
		t.Fatalf("unexpected row: %+v", got)
	}
}

func TestInterceptMissing(t *testing.T) {
	db := newTestDB(t)
	_, ok, err := GetIntercept(db, "nonexistent")
	if err != nil || ok {
		t.Fatalf("want ok=false nil err, got ok=%v err=%v", ok, err)
	}
}

func TestInterceptDelete(t *testing.T) {
	db := newTestDB(t)
	_ = InsertIntercept(db, sampleIntercept("evt-1"))
	if deleted, _ := DeleteIntercept(db, "evt-1"); !deleted {
		t.Fatal("should be deleted")
	}
	if _, ok, _ := GetIntercept(db, "evt-1"); ok {
		t.Fatal("should not exist after delete")
	}
}

func TestInterceptListOrderAndPagination(t *testing.T) {
	db := newTestDB(t)
	for i := 0; i < 5; i++ {
		r := sampleIntercept("evt-" + string(rune('a'+i)))
		r.Timestamp = "2026-08-04T10:0" + string(rune('0'+i)) + ":00Z"
		_ = InsertIntercept(db, r)
	}
	// 全量列表
	all, _ := ListIntercepts(db)
	if len(all) != 5 {
		t.Fatalf("want 5, got %d", len(all))
	}
	// 倒序校验:最新在前
	if all[0].ID != "evt-e" || all[4].ID != "evt-a" {
		t.Fatalf("wrong order: ids=%v", []string{all[0].ID, all[1].ID, all[2].ID, all[3].ID, all[4].ID})
	}
}
