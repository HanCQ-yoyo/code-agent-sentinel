// Package ruleengine 实现统一规则引擎的 schema 类型与 op 枚举。
// Task 2 仅定义类型与常量;校验(Validate)、求值(Eval)、加载(Load)在后续任务实现。
package ruleengine

import (
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
	regexes   map[string]CompiledRegex // key=op:field:value,规则级正则编译缓存(含 regexp2)
}

// MatchNode 保留 YAML 原始结构:叶子是 map{field,op,value},
// 布尔节点是 map{and/or/not: ...}。用 raw 存解码后的 map,
// 校验/求值时再解释。
type MatchNode struct {
	raw map[string]any
}

// ComboRule 是跨资产组合规则:所有 Requires 同时命中(AND)时产一条 Finding。
// 在 RulesDetector.Scan 的单资产循环后跑第二遍,输入是同 agent 的整个 []Asset。
// Requires[i].Match 复用 MatchNode + 现有 11 个 op;组合语义在 Requires 层面。
//
// 与单资产 Rule 的区别:ComboRule 的 AssetType 为空(组合规则不挂单一资产类型),
// 命中条件是"同批资产里每个 Require 各自找到一条满足的资产"(AND)。
type ComboRule struct {
	ID          string           `yaml:"id"`
	Severity    string           `yaml:"severity"`
	Description string           `yaml:"description"`
	Remediation string           `yaml:"remediation"`
	Metadata    map[string]any   `yaml:"metadata"`
	Requires    []ComboCondition `yaml:"requires"`
	Source      string           `yaml:"-"` // 来源文件路径(加载时填)
}

// ComboCondition 是 ComboRule 的一个子条件:在某类资产上匹配单条规则。
// AssetType 可空(=任意类型,求值时不过滤类型)。Match 复用 MatchNode 求值(evalLeaf)。
//
// compiled 是编译态(ValidateCombo 预填,不序列化):把本 require 当作单资产 Rule
// 编译,缓存正则进 compiled.regexes。Task 9 的 comboMatches 求值时构造 Rule
// 复用其 regexes 缓存,避免每资产重编译。comboMatches 用 req.AssetType 做 asset
// 类型路由(空=不过滤),不读 compiled.AssetType——故空 AssetType 的占位编译见
// ValidateCombo 注释。
type ComboCondition struct {
	AssetType string    `yaml:"asset_type"`
	Match     MatchNode `yaml:"match"`
	compiled  *Rule
}

// CompiledRule 返回 ValidateCombo 预编译的 Rule(含 regexes 缓存),供 security 包的
// comboMatches 复用正则缓存求值。未预编译(理论不发生,ValidateCombo 已编译)返回 nil。
//
// 跨包访问需求:comboMatches 在 internal/security(rules_detector.go, package security)
// 不能读 ComboCondition.compiled(未导出字段,跨包访问编译失败)。此导出方法是最小改动
// 解决方案,不破坏封装(callers 只读不能改)。
func (c ComboCondition) CompiledRule() *Rule { return c.compiled }

// Location 是 content 字段命中的文件位置(1-based)。
// Line=行号;StartCol/EndCol=字节列半开区间 [StartCol, EndCol),便于 Monaco 高亮。
// 仅 content 字段的 regex_match/contains 产生;字段级匹配与反混淆命中无位置。
type Location struct {
	Line     int `json:"line"`
	StartCol int `json:"start_col"`
	EndCol   int `json:"end_col"`
}

// EvalResult 是 Eval 的返回:是否命中 + 证据 + 命中位置列表。
type EvalResult struct {
	Matched   bool
	Evidence  string
	Locations []Location
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
