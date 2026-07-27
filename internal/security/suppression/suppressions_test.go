package suppression

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// Task 11 删除了 Suppressions.Save/Match(suppression 写路径已删)。
// 本测试覆盖只读 LoadSuppressions:文件不存在返回 (nil,nil);有效 YAML 返回数据。

func TestLoadSuppressionsMissingFileIsEmpty(t *testing.T) {
	s, err := LoadSuppressions("/nonexistent/path/suppressions.yaml")
	if err != nil {
		t.Fatalf("文件不存在应返回 (nil, nil), got err: %v", err)
	}
	if s != nil {
		t.Fatalf("文件不存在应返回 nil, got %+v", s)
	}
}

func TestLoadSuppressionsValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "suppressions.yaml")
	// 用 yaml.Marshal 直接写(不依赖已删的 Save)
	original := &Suppressions{Items: []Item{
		{Fingerprint: "fp1", Reason: "r1"},
		{RuleID: "injection.x", AssetID: "a1", Reason: "r2"},
		{RuleID: "baseline.y", Reason: "r3"},
	}}
	data, _ := yaml.Marshal(original)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadSuppressions(path)
	if err != nil {
		t.Fatalf("LoadSuppressions: %v", err)
	}
	if len(loaded.Items) != 3 {
		t.Fatalf("往返后 Items 数: got %d, want 3", len(loaded.Items))
	}
	if loaded.Items[0].Fingerprint != "fp1" || loaded.Items[0].Reason != "r1" {
		t.Errorf("item0: %+v", loaded.Items[0])
	}
}

func TestLoadSuppressionsInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "suppressions.yaml")
	if err := os.WriteFile(path, []byte("items: [invalid yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSuppressions(path); err == nil {
		t.Fatal("非法 YAML 应返回 error")
	}
}
