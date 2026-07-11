package suppression

import (
	"os"
	"testing"
)

// ── Contains 测试 ──

func TestBaselineContainsHitAndMiss(t *testing.T) {
	bs := &BaselineSet{Fingerprints: map[string]bool{"fp1": true, "fp2": true}}
	if !bs.Contains("fp1") {
		t.Fatal("已知指纹应命中")
	}
	if bs.Contains("unknown") {
		t.Fatal("未知指纹不应命中")
	}
}

func TestBaselineContainsNilSafe(t *testing.T) {
	// nil map 不应 panic
	var bs *BaselineSet
	if bs.Contains("anything") {
		t.Fatal("nil BaselineSet 的 Contains 应返回 false")
	}
}

// ── Load/Save 往返测试 ──

func TestBaselineSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/baseline.json"

	original := &BaselineSet{
		Version:      "1",
		GeneratedAt:  "2026-07-11",
		Fingerprints: map[string]bool{"fp1": true, "fp2": true},
	}
	if err := original.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadBaseline(path)
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}
	if loaded.Version != "1" {
		t.Errorf("Version: got %s, want 1", loaded.Version)
	}
	if loaded.GeneratedAt != "2026-07-11" {
		t.Errorf("GeneratedAt: got %s, want 2026-07-11", loaded.GeneratedAt)
	}
	if !loaded.Contains("fp1") || !loaded.Contains("fp2") {
		t.Fatal("往返后指纹应保留")
	}
	if loaded.Contains("fp3") {
		t.Fatal("未知指纹不应命中")
	}
}

// ── Save 创建父目录 ──

func TestBaselineSaveCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/nested/dir/baseline.json"

	bs := &BaselineSet{Fingerprints: map[string]bool{"fp1": true}}
	if err := bs.Save(path); err != nil {
		t.Fatalf("Save 应创建父目录: %v", err)
	}
	// 验证文件权限为 0o600
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("文件权限: got %o, want 0o600", perm)
	}
}

// ── LoadBaseline 文件不存在返回 (nil, nil) ──

func TestLoadBaselineMissingFileIsEmpty(t *testing.T) {
	bs, err := LoadBaseline("/nonexistent/path/baseline.json")
	if err != nil {
		t.Fatalf("文件不存在应返回 (nil, nil), got err: %v", err)
	}
	if bs != nil {
		t.Fatalf("文件不存在应返回 nil, got %+v", bs)
	}
}
