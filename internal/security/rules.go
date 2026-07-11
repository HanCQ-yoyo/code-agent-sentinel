package security

import (
	"embed"
	"regexp"

	"gopkg.in/yaml.v3"
)

//go:embed rules/*.yaml
var ruleFS embed.FS

type BaselineRule struct {
	ID          string         `yaml:"id"`
	Severity    Severity       `yaml:"severity"`
	Description string         `yaml:"description"`
	AssetType   string         `yaml:"asset_type"`
	Field       string         `yaml:"field"`
	Op          string         `yaml:"op"` // contains / key_matches / eq / true
	Value       string         `yaml:"value"`
	Remediation string         `yaml:"remediation"`
	Match       map[string]any `yaml:"match"` // 新 schema 向后兼容:Task 8 迁移后 field/op/value 在 match 树下
	re          *regexp.Regexp
}

type InjectionRule struct {
	ID            string         `yaml:"id"`
	Severity      Severity       `yaml:"severity"`
	Description   string         `yaml:"description"`
	AssetType     string         `yaml:"asset_type"`              // 新 schema:每条规则绑定一个 asset_type
	Pattern       string         `yaml:"pattern"`                 // 旧 schema 顶层;新 schema 从 match.value 提取
	Deobfuscation []string       `yaml:"deobfuscation"`
	Remediation   string         `yaml:"remediation"`
	Match         map[string]any `yaml:"match"` // 新 schema: {field: content, op: regex_match, value: <pattern>}
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
		// 新 schema 向后兼容:field/op/value 从 match 树提取
		if doc.Rules[i].Field == "" {
			if v, ok := doc.Rules[i].Match["field"].(string); ok {
				doc.Rules[i].Field = v
			}
		}
		if doc.Rules[i].Op == "" {
			if v, ok := doc.Rules[i].Match["op"].(string); ok {
				doc.Rules[i].Op = v
			}
		}
		if doc.Rules[i].Value == "" {
			if v, ok := doc.Rules[i].Match["value"].(string); ok {
				doc.Rules[i].Value = v
			}
		}
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
		// 新 schema 向后兼容:pattern 从 match.value 提取
		if doc.Rules[i].Pattern == "" {
			if v, ok := doc.Rules[i].Match["value"].(string); ok {
				doc.Rules[i].Pattern = v
			}
		}
		doc.Rules[i].re, err = regexp.Compile(doc.Rules[i].Pattern)
		if err != nil {
			return nil, err
		}
	}
	return doc.Rules, nil
}
