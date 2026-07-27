package configengine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMemoryOutlineExtracted(t *testing.T) {
	home := t.TempDir()
	claude := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claude, 0o700); err != nil {
		t.Fatal(err)
	}
	content := "# Title\nintro\n## Sub A\nbody\n### Sub A1\nmore\n## Sub B\nend"
	if err := os.WriteFile(filepath.Join(claude, "CLAUDE.md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	assets, err := parseMemory(claude, ScopeGlobal)
	if err != nil {
		t.Fatalf("parseMemory: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("assets = %d, want 1", len(assets))
	}
	outline, ok := assets[0].Fields["outline"].([]map[string]any)
	if !ok {
		t.Fatalf("outline not []map[string]any: %T", assets[0].Fields["outline"])
	}
	if len(outline) != 4 {
		t.Fatalf("outline len = %d, want 4", len(outline))
	}
	// # Title 在第 1 行
	if outline[0]["level"] != 1 || outline[0]["title"] != "Title" || outline[0]["line"] != 1 {
		t.Errorf("outline[0] = %+v", outline[0])
	}
	// ## Sub A 在第 3 行
	if outline[1]["level"] != 2 || outline[1]["title"] != "Sub A" || outline[1]["line"] != 3 {
		t.Errorf("outline[1] = %+v", outline[1])
	}
	// ### Sub A1 在第 5 行
	if outline[2]["level"] != 3 || outline[2]["title"] != "Sub A1" || outline[2]["line"] != 5 {
		t.Errorf("outline[2] = %+v", outline[2])
	}
}

func TestMemoryOutlineEmptyWhenNoHeadings(t *testing.T) {
	home := t.TempDir()
	claude := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claude, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claude, "CLAUDE.md"), []byte("no headings here\njust text"), 0o600); err != nil {
		t.Fatal(err)
	}
	assets, _ := parseMemory(claude, ScopeGlobal)
	outline, ok := assets[0].Fields["outline"].([]map[string]any)
	if !ok || len(outline) != 0 {
		t.Errorf("expected empty outline, got %v ok=%v", outline, ok)
	}
}
