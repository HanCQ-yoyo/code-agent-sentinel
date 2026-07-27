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
