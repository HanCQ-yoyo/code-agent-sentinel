package security

import (
	"context"
	"fmt"
	"strings"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security/ruleengine"
)

type InjectionDetector struct{ rules []InjectionRule }

func NewInjectionDetector() *InjectionDetector {
	r, _ := loadInjectionRules()
	return &InjectionDetector{rules: r}
}

func (d *InjectionDetector) ID() string { return "content.injection" }
func (d *InjectionDetector) Covers() []configengine.AssetType {
	return []configengine.AssetType{
		configengine.AssetMCPServer, configengine.AssetSkill, configengine.AssetCommand,
		configengine.AssetAgent, configengine.AssetMemory, configengine.AssetScript,
	}
}
func (d *InjectionDetector) Available() bool { return true }
func (d *InjectionDetector) Reason() string  { return "" }

func (d *InjectionDetector) Meta() DetectorMeta {
	rules := make([]RuleInfo, 0, len(d.rules))
	for _, r := range d.rules {
		rules = append(rules, RuleInfo{
			ID:          r.ID,
			Severity:    string(r.Severity),
			Description: r.Description,
			Syntax:      r.Pattern,
		})
	}
	covers := make([]string, 0, len(d.Covers()))
	for _, c := range d.Covers() {
		covers = append(covers, string(c))
	}
	return DetectorMeta{
		ID:      d.ID(),
		Name:    "提示注入检测",
		Engines: []EngineInfo{{Name: "内嵌 YAML 引擎 + 反混淆(zero_width/html_comment/base64/leetspeak)", Kind: "embedded", Available: true}},
		Rules:   rules,
		Covers:  covers,
	}
}

func (d *InjectionDetector) Scan(ctx context.Context, assets []configengine.Asset) ([]Finding, error) {
	var out []Finding
	for _, a := range assets {
		text := assetText(a)
		if text == "" {
			continue
		}
		for _, r := range d.rules {
			for _, variant := range ruleengine.Deobfuscate(text, r.Deobfuscation) {
				if r.re != nil && r.re.MatchString(variant.Text) {
					out = append(out, Finding{
						DetectorID:  d.ID(),
						RuleID:      r.ID,
						Severity:    r.Severity,
						AssetID:     a.ID,
						AssetType:   a.Type,
						AssetName:   a.Name,
						Message:     r.Description,
						Evidence:    truncate(r.re.FindString(variant.Text), 200),
						Remediation: r.Remediation,
					})
					break // 同一规则同一资产只报一次
				}
			}
		}
	}
	return out, nil
}

func assetText(a configengine.Asset) string {
	var b strings.Builder
	if a.Content != "" {
		b.WriteString(a.Content)
	}
	for _, v := range a.Fields {
		if s, ok := v.(string); ok && s != "" {
			b.WriteString("\n" + s)
		}
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// 占位避免 unused(若 fmt 未用)
var _ = fmt.Sprint
