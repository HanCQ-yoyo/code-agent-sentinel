package security

import "testing"

func TestLoadBaselineRules(t *testing.T) {
	rs, err := loadBaselineRules()
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) < 3 {
		t.Fatalf("规则太少: %d", len(rs))
	}
	if rs[0].ID == "" || rs[0].Severity == "" {
		t.Errorf("规则字段缺失: %+v", rs[0])
	}
}

func TestLoadInjectionRules(t *testing.T) {
	rs, err := loadInjectionRules()
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) < 2 {
		t.Fatalf("规则太少: %d", len(rs))
	}
}
