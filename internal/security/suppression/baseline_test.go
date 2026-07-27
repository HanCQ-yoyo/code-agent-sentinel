package suppression

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Task 11 删除了 BaselineSet.Save/Contains(suppression 写路径已删)。
// 本测试覆盖只读 LoadBaseline:文件不存在返回 (nil,nil);有效 JSON 返回数据。

func TestLoadBaselineMissingFileIsEmpty(t *testing.T) {
	bs, err := LoadBaseline("/nonexistent/path/baseline.json")
	if err != nil {
		t.Fatalf("文件不存在应返回 (nil, nil), got err: %v", err)
	}
	if bs != nil {
		t.Fatalf("文件不存在应返回 nil, got %+v", bs)
	}
}

func TestLoadBaselineValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")
	// 用 encoding/json 直接写(不依赖已删的 Save)
	original := &BaselineSet{
		Version:      "1",
		GeneratedAt:  "2026-07-11",
		Fingerprints: map[string]bool{"fp1": true, "fp2": true},
	}
	data, _ := json.MarshalIndent(original, "", "  ")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadBaseline(path)
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}
	if loaded.Version != "1" {
		t.Errorf("Version: got %s, want 1", loaded.Version)
	}
	if len(loaded.Fingerprints) != 2 || !loaded.Fingerprints["fp1"] || !loaded.Fingerprints["fp2"] {
		t.Errorf("Fingerprints 往返不符: %+v", loaded.Fingerprints)
	}
}

func TestLoadBaselineInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBaseline(path); err == nil {
		t.Fatal("非法 JSON 应返回 error")
	}
}
