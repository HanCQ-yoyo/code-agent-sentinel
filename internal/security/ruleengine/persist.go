package ruleengine

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// RuleRow 是一条 Rule 的扁平可序列化表示(对应 sqlite 规则表一行)。
// map/slice 字段统一以 JSON 文本列存储(MatchJSON 等),与 DB 列直接同构,
// 避免在 Row 与 DB 列之间再加一层中间类型。RuleToRow 负责序列化,
// RowToRule 负责反序列化,二者经 MatchNode.Raw()/NewMatchNode() 保持与
// YAML 加载路径(parseRuleYAML → UnmarshalYAML)同构,从而往返后
// Validate→Eval 行为等价(Task 7 等价性测试的门)。
//
// 不进 Row 的字段:Source/ProjectPath(运行时加载态)、编译态(assetType/regexes)。
type RuleRow struct {
	ID                string `json:"id"`
	Severity          string `json:"severity"`
	AssetType         string `json:"asset_type"`
	MatchJSON         string `json:"match_json"`    // MatchNode.Raw() 的 JSON 文本,空=无 match
	DeobfuscationJSON string `json:"deobfuscation"` // JSON 数组文本,空为 ""
	Dotall            bool   `json:"dotall"`
	PathsJSON         string `json:"paths_json"`   // PathFilter 的 JSON 文本,空=nil
	PostExcludeJSON   string `json:"post_exclude"` // JSON 数组文本,空为 ""
	Remediation       string `json:"remediation"`
	Description       string `json:"description"`
	MetadataJSON      string `json:"metadata_json"` // map JSON 文本,空为 ""
}

// ComboRow 是一条 ComboRule 的扁平可序列化表示(对应 sqlite combo 表一行)。
// RequiresJSON 是 [{asset_type, match:{...}}, ...] 的 JSON 数组文本。
type ComboRow struct {
	ID           string `json:"id"`
	Severity     string `json:"severity"`
	Description  string `json:"description"`
	Remediation  string `json:"remediation"`
	MetadataJSON string `json:"metadata_json"`
	RequiresJSON string `json:"requires_json"`
}

// comboRequireItem 是 ComboCondition 落库的单元素:asset_type + match 的原始 map。
type comboRequireItem struct {
	AssetType string         `json:"asset_type"`
	Match     map[string]any `json:"match"`
}

// RuleToRow 把 Rule 转成 RuleRow(可序列化)。MatchNode 经 .Raw() 取原始 map 后
// JSON 序列化;Paths/Deobfuscation/PostExclude/Metadata 同理。Source/ProjectPath/
// 编译态字段不进 Row(它们是运行时/加载态)。
func RuleToRow(r Rule) (RuleRow, error) {
	matchJSON, err := jsonMarshalMap(r.Match.Raw())
	if err != nil {
		return RuleRow{}, fmt.Errorf("marshal match: %w", err)
	}
	pathsJSON, err := jsonMarshalPtr(r.Paths)
	if err != nil {
		return RuleRow{}, fmt.Errorf("marshal paths: %w", err)
	}
	deobJSON, err := jsonMarshalStrings(r.Deobfuscation)
	if err != nil {
		return RuleRow{}, fmt.Errorf("marshal deobfuscation: %w", err)
	}
	postExcludeJSON, err := jsonMarshalStrings(r.PostExclude)
	if err != nil {
		return RuleRow{}, fmt.Errorf("marshal post_exclude: %w", err)
	}
	metaJSON, err := jsonMarshalMap(r.Metadata)
	if err != nil {
		return RuleRow{}, fmt.Errorf("marshal metadata: %w", err)
	}
	return RuleRow{
		ID:                r.ID,
		Severity:          r.Severity,
		AssetType:         r.AssetType,
		MatchJSON:         matchJSON,
		DeobfuscationJSON: deobJSON,
		Dotall:            r.Dotall,
		PathsJSON:         pathsJSON,
		PostExcludeJSON:   postExcludeJSON,
		Remediation:       r.Remediation,
		Description:       r.Description,
		MetadataJSON:      metaJSON,
	}, nil
}

// RowToRule 把 RuleRow 还原成 Rule。MatchNode 用 NewMatchNode 构造(不经 yaml),
// 与 embed 加载路径(UnmarshalYAML→raw map)产出同构结构;Validate 会重新编译
// assetType/regexes,故此处不填编译态(与 parseRuleYAML→Validate 一致)。
// AssetType 直接赋值:Rule.AssetType 本就是 string(非 configengine.AssetType)。
func RowToRule(row RuleRow) (Rule, error) {
	var match map[string]any
	if row.MatchJSON != "" {
		if err := json.Unmarshal([]byte(row.MatchJSON), &match); err != nil {
			return Rule{}, fmt.Errorf("unmarshal match_json: %w", err)
		}
	}
	var paths *PathFilter
	if row.PathsJSON != "" {
		var p PathFilter
		if err := json.Unmarshal([]byte(row.PathsJSON), &p); err != nil {
			return Rule{}, fmt.Errorf("unmarshal paths_json: %w", err)
		}
		paths = &p
	}
	var deob []string
	if row.DeobfuscationJSON != "" {
		if err := json.Unmarshal([]byte(row.DeobfuscationJSON), &deob); err != nil {
			return Rule{}, fmt.Errorf("unmarshal deobfuscation: %w", err)
		}
	}
	var postExclude []string
	if row.PostExcludeJSON != "" {
		if err := json.Unmarshal([]byte(row.PostExcludeJSON), &postExclude); err != nil {
			return Rule{}, fmt.Errorf("unmarshal post_exclude: %w", err)
		}
	}
	var metadata map[string]any
	if row.MetadataJSON != "" {
		if err := json.Unmarshal([]byte(row.MetadataJSON), &metadata); err != nil {
			return Rule{}, fmt.Errorf("unmarshal metadata_json: %w", err)
		}
	}
	return Rule{
		ID:            row.ID,
		Severity:      row.Severity,
		AssetType:     row.AssetType,
		Match:         NewMatchNode(match),
		Deobfuscation: deob,
		Dotall:        row.Dotall,
		Paths:         paths,
		PostExclude:   postExclude,
		Remediation:   row.Remediation,
		Description:   row.Description,
		Metadata:      metadata,
	}, nil
}

// RowToMatchJSON 从 RuleRow 取出落库用的 JSON 文本字段(match_json 等)。
// RuleRow 已以 JSON 文本列存储,故直接返回对应字段(Task 4 存储层写库时调用)。
func RowToMatchJSON(row RuleRow) (matchJSON, pathsJSON, deobJSON, postExcludeJSON, metaJSON string, err error) {
	return row.MatchJSON, row.PathsJSON, row.DeobfuscationJSON, row.PostExcludeJSON, row.MetadataJSON, nil
}

// RowFromDBColumns 从 db 查询列文本构造 RuleRow(match_json 等是 JSON 文本)。
// dotall 是 0/1 整数(SQLite BOOLEAN)。各 JSON 列空串表示 nil/空切片。
func RowFromDBColumns(id, severity, assetType, matchJSON, pathsJSON, deobJSON string, dotall int,
	postExcludeJSON, remediation, description, metaJSON string) (RuleRow, error) {
	return RuleRow{
		ID:                id,
		Severity:          severity,
		AssetType:         assetType,
		MatchJSON:         matchJSON,
		PathsJSON:         pathsJSON,
		DeobfuscationJSON: deobJSON,
		Dotall:            dotall != 0,
		PostExcludeJSON:   postExcludeJSON,
		Remediation:       remediation,
		Description:       description,
		MetadataJSON:      metaJSON,
	}, nil
}

// ComboToRow 把 ComboRule 转成 ComboRow。Requires 逐条取 .Match.Raw() 落为
// comboRequireItem 后整体 JSON 序列化。Source 不进 Row(运行时加载态)。
func ComboToRow(cr ComboRule) (ComboRow, error) {
	items := make([]comboRequireItem, len(cr.Requires))
	for i, req := range cr.Requires {
		items[i] = comboRequireItem{
			AssetType: req.AssetType,
			Match:     req.Match.Raw(),
		}
	}
	requiresJSON := ""
	if len(items) > 0 {
		b, err := json.Marshal(items)
		if err != nil {
			return ComboRow{}, fmt.Errorf("marshal requires: %w", err)
		}
		requiresJSON = string(b)
	}
	metaJSON, err := jsonMarshalMap(cr.Metadata)
	if err != nil {
		return ComboRow{}, fmt.Errorf("marshal metadata: %w", err)
	}
	return ComboRow{
		ID:           cr.ID,
		Severity:     cr.Severity,
		Description:  cr.Description,
		Remediation:  cr.Remediation,
		MetadataJSON: metaJSON,
		RequiresJSON: requiresJSON,
	}, nil
}

// RowToCombo 把 ComboRow 还原成 ComboRule。Requires 经 JSON 反序列化为
// comboRequireItem 列表,每个用 NewMatchNode 构造 MatchNode。compiled 字段
// 不填(由 ValidateCombo 预编译,与单资产 Rule 同理)。
func RowToCombo(row ComboRow) (ComboRule, error) {
	var items []comboRequireItem
	if row.RequiresJSON != "" {
		if err := json.Unmarshal([]byte(row.RequiresJSON), &items); err != nil {
			return ComboRule{}, fmt.Errorf("unmarshal requires_json: %w", err)
		}
	}
	var metadata map[string]any
	if row.MetadataJSON != "" {
		if err := json.Unmarshal([]byte(row.MetadataJSON), &metadata); err != nil {
			return ComboRule{}, fmt.Errorf("unmarshal metadata_json: %w", err)
		}
	}
	requires := make([]ComboCondition, len(items))
	for i, item := range items {
		requires[i] = ComboCondition{
			AssetType: item.AssetType,
			Match:     NewMatchNode(item.Match),
		}
	}
	return ComboRule{
		ID:          row.ID,
		Severity:    row.Severity,
		Description: row.Description,
		Remediation: row.Remediation,
		Metadata:    metadata,
		Requires:    requires,
	}, nil
}

// jsonMarshalMap 把 map 序列化为 JSON 文本(nil/空 → "")。
func jsonMarshalMap(m map[string]any) (string, error) {
	if len(m) == 0 {
		return "", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// jsonMarshalStrings 把 []string 序列化为 JSON 文本(nil/空 → "")。
func jsonMarshalStrings(s []string) (string, error) {
	if len(s) == 0 {
		return "", nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// jsonMarshalPtr 把 *PathFilter 序列化为 JSON 文本(nil → "")。
func jsonMarshalPtr(p *PathFilter) (string, error) {
	if p == nil {
		return "", nil
	}
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// yamlUnmarshal 是 yaml.Unmarshal 的薄包装(测试与 persist 共用 *ruleFile 入口)。
// 与 parseRuleYAML 同构,确保测试经此路径构造的 Rule 与生产加载路径一致。
func yamlUnmarshal(data []byte, out *ruleFile) error {
	return yaml.Unmarshal(data, out)
}
