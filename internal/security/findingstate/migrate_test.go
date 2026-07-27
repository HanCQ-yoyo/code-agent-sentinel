package findingstate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/security/suppression"
	"gopkg.in/yaml.v3"
)

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeYAML(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	b, _ := yaml.Marshal(v)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateFromLegacyBoth(t *testing.T) {
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "baseline.json")
	suppPath := filepath.Join(dir, "suppressions.yaml")
	statesPath := filepath.Join(dir, "finding_states.yaml")

	writeJSON(t, baselinePath, suppression.BaselineSet{Fingerprints: map[string]bool{"fp_b1": true, "fp_b2": true}})
	writeYAML(t, suppPath, suppression.Suppressions{Items: []suppression.Item{
		{Fingerprint: "fp_s1", Reason: "重定向到 /tmp"},
		{RuleID: "destructive.foo", Reason: "全局规则,请到规则配置禁用"}, // rule_id 全局项
	}})

	rep, err := MigrateFromLegacy(baselinePath, suppPath, statesPath)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if rep.BaselineCount != 2 || rep.InlineCount != 1 || rep.GlobalRuleDropped != 1 {
		t.Errorf("report = %+v", rep)
	}

	loaded, err := Load(statesPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// 2 baseline + 1 inline(fingerprint)= 3 条 state
	if len(loaded.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(loaded.Items))
	}
	for _, it := range loaded.Items {
		if it.Status != StatusAccepted {
			t.Errorf("item %s status = %s, want accepted", it.Fingerprint, it.Status)
		}
		if it.Source != SourceMigratedBaseline && it.Source != SourceMigratedInline {
			t.Errorf("item %s source = %s", it.Fingerprint, it.Source)
		}
	}

	// 旧文件重命名 .legacy
	if _, err := os.Stat(baselinePath + ".legacy"); err != nil {
		t.Errorf("baseline.json.legacy missing: %v", err)
	}
	if _, err := os.Stat(suppPath + ".legacy"); err != nil {
		t.Errorf("suppressions.yaml.legacy missing: %v", err)
	}
}

func TestMigrateFromLegacyNoLegacyFiles(t *testing.T) {
	dir := t.TempDir()
	rep, err := MigrateFromLegacy(
		filepath.Join(dir, "baseline.json"),
		filepath.Join(dir, "suppressions.yaml"),
		filepath.Join(dir, "finding_states.yaml"))
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if rep.BaselineCount != 0 || rep.InlineCount != 0 {
		t.Errorf("expected zero report, got %+v", rep)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	// statesPath 已存在 → 不迁移(避免覆盖用户已有处置)
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "baseline.json")
	statesPath := filepath.Join(dir, "finding_states.yaml")
	writeJSON(t, baselinePath, suppression.BaselineSet{Fingerprints: map[string]bool{"fp_b1": true}})
	// 预先创建 statesPath
	writeYAML(t, statesPath, States{Items: []State{{Fingerprint: "existing", Status: StatusResolved}}})

	rep, err := MigrateFromLegacy(baselinePath, filepath.Join(dir, "suppressions.yaml"), statesPath)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if rep.Skipped {
		// 允许返回 Skipped=true 表示跳过
	} else if rep.BaselineCount != 0 {
		t.Errorf("should skip when states exist, got %+v", rep)
	}
}
