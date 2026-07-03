package security

import (
	"embed"
	"regexp"

	"gopkg.in/yaml.v3"
)

//go:embed rules/*.yaml
var ruleFS embed.FS

type BaselineRule struct {
	ID          string   `yaml:"id"`
	Severity    Severity `yaml:"severity"`
	Description string   `yaml:"description"`
	AssetType   string   `yaml:"asset_type"`
	Field       string   `yaml:"field"`
	Op          string   `yaml:"op"` // contains / key_matches / eq / true
	Value       string   `yaml:"value"`
	Remediation string   `yaml:"remediation"`
	re          *regexp.Regexp
}

type InjectionRule struct {
	ID            string   `yaml:"id"`
	Severity      Severity `yaml:"severity"`
	Description   string   `yaml:"description"`
	Pattern       string   `yaml:"pattern"`
	Deobfuscation []string `yaml:"deobfuscation"`
	Remediation   string   `yaml:"remediation"`
	re            *regexp.Regexp
}

func loadBaselineRules() ([]BaselineRule, error) {
	data, err := ruleFS.ReadFile("rules/baseline.yaml")
	if err != nil {
		return nil, err
	}
	var doc struct {
		Rules []BaselineRule `yaml:"rules"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	for i := range doc.Rules {
		if doc.Rules[i].Op == "key_matches" || doc.Rules[i].Op == "matches" {
			doc.Rules[i].re, err = regexp.Compile(doc.Rules[i].Value)
			if err != nil {
				return nil, err
			}
		}
	}
	return doc.Rules, nil
}

func loadInjectionRules() ([]InjectionRule, error) {
	data, err := ruleFS.ReadFile("rules/injection.yaml")
	if err != nil {
		return nil, err
	}
	var doc struct {
		Rules []InjectionRule `yaml:"rules"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	for i := range doc.Rules {
		doc.Rules[i].re, err = regexp.Compile(doc.Rules[i].Pattern)
		if err != nil {
			return nil, err
		}
	}
	return doc.Rules, nil
}
