package ruleengine

import "testing"

func TestCategoryOf(t *testing.T) {
	cases := []struct {
		ruleID string
		want   Category
	}{
		{"destructive.filesystem.redirect-truncate-dynamic-path", CategoryDestructiveCommand},
		{"destructive.git.reset-hard", CategoryDestructiveCommand},
		{"injection.prompt-injection.p1", CategoryPromptInjection},
		{"injection.anti-refusal.ar1", CategoryPromptInjection},
		{"injection.memory-poisoning.mp2", CategoryPromptInjection},
		{"baseline.dangerous-skip-permission", CategoryBaseline},
		{"skill.description-hidden-instruction", CategorySkillAbuse},
		{"combo.skip-perm-with-bash-wildcard", CategoryCombo},
		{"rules.load-error", CategoryLoadError},
		{"", CategoryUnknown},
		{"weird-thing", CategoryUnknown},
	}
	for _, c := range cases {
		if got := CategoryOf(c.ruleID); got != c.want {
			t.Errorf("CategoryOf(%q) = %q, want %q", c.ruleID, got, c.want)
		}
	}
}

func TestCategorySubclassInjection(t *testing.T) {
	// injection 第二段是 SkillSpector 威胁子类,前端字典可展
	if sub := InjectionSubclass("injection.data-exfiltration.x1"); sub != "data-exfiltration" {
		t.Errorf("subclass = %q, want data-exfiltration", sub)
	}
	if sub := InjectionSubclass("injection.prompt-injection.p1"); sub != "prompt-injection" {
		t.Errorf("subclass = %q, want prompt-injection", sub)
	}
	if sub := InjectionSubclass("baseline.foo"); sub != "" {
		t.Errorf("non-injection subclass = %q, want empty", sub)
	}
}
