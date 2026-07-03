package security

import (
	"context"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

func TestBaselineDetectsWildcardBash(t *testing.T) {
	d := NewBaselineDetector()
	perm := configengine.Asset{ID: "p1", Type: configengine.AssetPermissions, Name: "permissions"}
	perm.Fields = map[string]any{"allow": []any{"Bash(*)"}}
	findings, err := d.Scan(context.Background(), []configengine.Asset{perm})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "baseline.wildcard-bash" {
			found = true
		}
	}
	if !found {
		t.Errorf("未检出通配 Bash: %+v", findings)
	}
}

func TestBaselineDetectsSkipPermission(t *testing.T) {
	d := NewBaselineDetector()
	s := configengine.Asset{ID: "s1", Type: configengine.AssetSettings, Name: "settings"}
	s.Fields = map[string]any{"raw": []byte(`{"skipDangerousModePermissionPrompt":true}`)}
	findings, _ := d.Scan(context.Background(), []configengine.Asset{s})
	ok := false
	for _, f := range findings {
		if f.RuleID == "baseline.dangerous-skip-permission" {
			ok = true
		}
	}
	if !ok {
		t.Errorf("未检出 skipDangerous: %+v", findings)
	}
}
