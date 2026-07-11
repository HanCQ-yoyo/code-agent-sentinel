// Package ruleengine 实现统一规则引擎的 schema 类型与 op 枚举。
// Task 2 仅定义类型与常量;校验(Validate)、求值(Eval)、加载(Load)在后续任务实现。
package ruleengine

import (
	"regexp"

	"code-agent-sentinel/internal/configengine"
	"gopkg.in/yaml.v3"
)

// op 枚举(11 个,6 类)
const (
	OpExists        = "exists"
	OpNotExists     = "not_exists"
	OpEq            = "eq"
	OpNotEquals     = "not_equals"
	OpContains      = "contains"
	OpNotContains   = "not_contains"
	OpRegexMatch    = "regex_match"
	OpNotRegexMatch = "not_regex_match"
	OpKeyMatches    = "key_matches"
	OpWithin        = "within"
	OpNotWithin     = "not_within"
	// 规则级特殊求值模式(求值器内置,非用户 op)
	SpecialRepeat    = "repeat_check"
	SpecialHomoglyph = "homoglyph_check"
)

// validOp 判断 op 是否为 11 个用户可用 op 之一。
// 特殊求值模式(SpecialRepeat/SpecialHomoglyph)不算用户 op。
func validOp(op string) bool {
	switch op {
	case OpExists, OpNotExists, OpEq, OpNotEquals, OpContains, OpNotContains,
		OpRegexMatch, OpNotRegexMatch, OpKeyMatches, OpWithin, OpNotWithin:
		return true
	}
	return false
}

// Rule 是一条已加载(可能已校验)的规则。
type Rule struct {
	ID            string         `yaml:"id"`
	Severity      string         `yaml:"severity"`
	AssetType     string         `yaml:"asset_type"`
	Match         MatchNode      `yaml:"match"` // 可为 nil(禁用)
	Deobfuscation []string       `yaml:"deobfuscation"`
	Dotall        bool           `yaml:"dotall"`
	Paths         *PathFilter    `yaml:"paths"`
	PostExclude   []string       `yaml:"post_exclude"` // RE2 改写用:命中后排除上下文
	Remediation   string         `yaml:"remediation"`
	Description   string         `yaml:"description"`
	Metadata      map[string]any `yaml:"metadata"`
	Source        string         `yaml:"-"` // 来源文件路径(加载时填)
	// ProjectPath 用于项目隔离:项目规则带此字段,求值时只对 SourcePath 在该项目下的资产生效。
	// 内置/全局规则此字段为空,对所有资产生效。由加载器按规则来源目录设置,非 YAML 字段。
	// (Task 11 RulesDetector 在 Scan 循环里检查 pathInProject(asset.SourcePath, r.ProjectPath)。)
	ProjectPath string `yaml:"-"`
	// 编译态(校验时填,不序列化)
	assetType configengine.AssetType
	regexes   map[string]*regexp.Regexp // key=op:field:value,规则级正则编译缓存
}

// MatchNode 保留 YAML 原始结构:叶子是 map{field,op,value},
// 布尔节点是 map{and/or/not: ...}。用 raw 存解码后的 map,
// 校验/求值时再解释。
type MatchNode struct {
	raw map[string]any
}

// UnmarshalYAML 将 YAML 节点解码到 raw map,保留原始结构。
func (m *MatchNode) UnmarshalYAML(node *yaml.Node) error {
	var v map[string]any
	if err := node.Decode(&v); err != nil {
		return err
	}
	m.raw = v
	return nil
}

// PathFilter 按路径包含/排除过滤资产。
type PathFilter struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

// RuleLoadError 表示一条规则加载/校验失败的信息。
type RuleLoadError struct {
	Source string // 来源文件路径
	RuleID string // 规则 ID(可为空,如 YAML 语法错误时)
	Reason string // 失败原因
}
