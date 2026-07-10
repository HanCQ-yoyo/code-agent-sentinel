package editor

import (
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

func TestNewEditor(t *testing.T) {
	eng := configengine.NewEngine(t.TempDir())
	e := New(eng, "/tmp/backups", 20)
	if e == nil || e.Engine == nil {
		t.Fatal("New returned nil or nil Engine")
	}
	if e.BackupDir != "/tmp/backups" || e.MaxBackups != 20 {
		t.Fatalf("got BackupDir=%q MaxBackups=%d", e.BackupDir, e.MaxBackups)
	}
}

func TestNewEditorDefaults(t *testing.T) {
	eng := configengine.NewEngine(t.TempDir())
	e := New(eng, "", 0)
	if e.MaxBackups != 20 {
		t.Fatalf("default MaxBackups want 20 got %d", e.MaxBackups)
	}
	// BackupDir 空时默认 ~/.claude-sentinel/backups(home 之下 .claude-sentinel)
	want := filepath.Join(eng.HomeDir, ".claude-sentinel", "backups")
	if e.BackupDir != want {
		t.Fatalf("default BackupDir want %q got %q", want, e.BackupDir)
	}
}

func TestErrorsAreSentinels(t *testing.T) {
	if ErrConcurrentModification == nil || ErrNotEditable == nil || ErrOutOfRoot == nil || ErrBadContent == nil {
		t.Fatal("sentinel errors must be non-nil")
	}
}
