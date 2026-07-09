package security

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"code-agent-sentinel/internal/configengine"
)

type BaselineDetector struct{ rules []BaselineRule }

func NewBaselineDetector() *BaselineDetector {
	r, _ := loadBaselineRules()
	return &BaselineDetector{rules: r}
}

func (d *BaselineDetector) ID() string { return "baseline" }
func (d *BaselineDetector) Covers() []configengine.AssetType {
	return []configengine.AssetType{configengine.AssetSettings, configengine.AssetPermissions}
}
func (d *BaselineDetector) Available() bool { return true }
func (d *BaselineDetector) Reason() string  { return "" }

func (d *BaselineDetector) Meta() DetectorMeta {
	rules := make([]RuleInfo, 0, len(d.rules))
	for _, r := range d.rules {
		rules = append(rules, RuleInfo{
			ID:          r.ID,
			Severity:    string(r.Severity),
			Description: r.Description,
			Syntax:      baselineSyntax(r),
		})
	}
	covers := make([]string, 0, len(d.Covers()))
	for _, c := range d.Covers() {
		covers = append(covers, string(c))
	}
	return DetectorMeta{
		ID:      d.ID(),
		Name:    "基线检测",
		Engines: []EngineInfo{{Name: "内嵌 YAML 规则引擎", Kind: "embedded", Available: true}},
		Rules:   rules,
		Covers:  covers,
	}
}

// baselineSyntax 按 op 生成可读规则语法。contains → 字段包含 value;
// key_matches → 键匹配正则;兜底 → op value 形式。
func baselineSyntax(r BaselineRule) string {
	switch r.Op {
	case "contains":
		return fmt.Sprintf("字段 %q 包含 %q", r.Field, r.Value)
	case "key_matches":
		return fmt.Sprintf("键 %q 匹配正则 /%s/", r.Field, r.Value)
	default:
		return fmt.Sprintf("字段 %q %s %q", r.Field, r.Op, r.Value)
	}
}

func (d *BaselineDetector) Scan(ctx context.Context, assets []configengine.Asset) ([]Finding, error) {
	var out []Finding
	for _, a := range assets {
		for _, r := range d.rules {
			if !ruleAppliesToAsset(r, a) {
				continue
			}
			if matched, evidence := evalBaselineRule(r, a); matched {
				out = append(out, Finding{
					DetectorID:  d.ID(),
					RuleID:      r.ID,
					Severity:    r.Severity,
					AssetID:     a.ID,
					AssetType:   a.Type,
					AssetName:   a.Name,
					Message:     r.Description,
					Evidence:    evidence,
					Remediation: r.Remediation,
				})
			}
		}
	}
	return out, nil
}

func ruleAppliesToAsset(r BaselineRule, a configengine.Asset) bool {
	return r.AssetType == string(a.Type)
}

func evalBaselineRule(r BaselineRule, a configengine.Asset) (bool, string) {
	val, ok := a.Fields[r.Field]
	if !ok {
		return false, ""
	}
	switch r.Op {
	case "contains":
		s := stringify(val)
		if strings.Contains(s, r.Value) {
			return true, fmt.Sprintf("%s contains %q", r.Field, r.Value)
		}
	case "key_matches":
		// val 是 map[string]any 或 map[string]string
		keys := mapKeys(val)
		for _, k := range keys {
			if r.re != nil && r.re.MatchString(k) {
				return true, fmt.Sprintf("env key %q matches %s", k, r.Value)
			}
		}
	}
	return false, ""
}

func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case json.RawMessage:
		return string(t)
	case []any:
		var parts []string
		for _, x := range t {
			parts = append(parts, fmt.Sprint(x))
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprint(v)
	}
}

func mapKeys(v any) []string {
	var keys []string
	switch t := v.(type) {
	case map[string]any:
		for k := range t {
			keys = append(keys, k)
		}
	case map[string]string:
		for k := range t {
			keys = append(keys, k)
		}
	}
	return keys
}
