package storage

import (
	"testing"
)

func TestAllowlistReplaceAndList(t *testing.T) {
	db := newTestDB(t)
	entries := []AllowlistEntryRow{
		{Command: "ls -la"},
		{Command: "git status"},
		{Command: "npm install"},
	}
	if err := ReplaceAllowlist(db, entries); err != nil {
		t.Fatalf("ReplaceAllowlist: %v", err)
	}
	got, err := ListAllowlist(db)
	if err != nil {
		t.Fatalf("ListAllowlist: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 entries, got %d", len(got))
	}
	// 验证排序和内容
	if got[0].Command != "git status" || got[1].Command != "ls -la" || got[2].Command != "npm install" {
		t.Fatalf("unexpected order: %+v", got)
	}
}

func TestAllowlistReplaceOverwrites(t *testing.T) {
	db := newTestDB(t)
	if err := ReplaceAllowlist(db, []AllowlistEntryRow{{Command: "old"}}); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceAllowlist(db, []AllowlistEntryRow{{Command: "new"}}); err != nil {
		t.Fatal(err)
	}
	got, _ := ListAllowlist(db)
	if len(got) != 1 || got[0].Command != "new" {
		t.Fatalf("replace should overwrite: %+v", got)
	}
}

func TestAllowlistEmptyReplace(t *testing.T) {
	db := newTestDB(t)
	// 空列表替换应清空表
	if err := ReplaceAllowlist(db, []AllowlistEntryRow{{Command: "x"}}); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceAllowlist(db, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := ListAllowlist(db)
	if len(got) != 0 {
		t.Fatalf("empty replace should clear: got %d", len(got))
	}
}
