package ruleengine

import (
	"fmt"
	"path/filepath"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/storage"
)

// LoadDetectRules 从 sqlite 加载检测规则(热重载:每次调用实时读 db):
//  1. db 读 detect_rules LEFT JOIN detect_overrides,SQL 层过滤 enabled=false 的。
//  2. builtin+custom 规则合并(同 id 后者覆盖,经 Merge)。
//  3. 叠加项目级 .sentinel/rules 文件规则(经 LoadDir,带 ProjectPath)。
//  4. Validate(编译正则)。
//  5. combos 从 detect_combos 读(builtin only)。
//
// 本函数取代文件加载路径 LoadForScan 用于 DB 模式(LoadForScan 暂留,Task 13 废弃)。
// Finding #5(硬编码路径)随此闭合:统一读 db,不再假定 ~/.claude-sentinel。
func LoadDetectRules(db *storage.DB, inv *configengine.Inventory) (rules []Rule, combos []ComboRule, errs []RuleLoadError) {
	if db == nil {
		errs = append(errs, RuleLoadError{Source: "db", Reason: "detect rules db is nil"})
		return nil, nil, errs
	}

	// 1+2. 读 detect_rules + overrides(enabled 过滤在 SQL)
	stored, err := storage.ListRulesEnabled(db, storage.DomainDetect)
	if err != nil {
		errs = append(errs, RuleLoadError{Source: "db:detect", Reason: fmt.Sprintf("list rules: %v", err)})
		return nil, nil, errs
	}
	var dbRules []Rule
	for _, s := range stored {
		r, convErr := StoredRuleToRule(s)
		if convErr != nil {
			errs = append(errs, RuleLoadError{Source: "db:detect:" + s.ID, Reason: convErr.Error()})
			continue
		}
		dbRules = append(dbRules, r)
	}

	// 3. 项目级规则(文件,带 ProjectPath)。项目 combo 暂不接(与 LoadForScan 同语义)。
	var projectRules []Rule
	if inv != nil {
		for _, p := range inv.Projects {
			dir := filepath.Join(p.Path, ".sentinel", "rules")
			prules, _, perrs := LoadDir(dir, "project")
			errs = append(errs, perrs...)
			for i := range prules {
				prules[i].ProjectPath = p.Path
			}
			projectRules = append(projectRules, prules...)
		}
	}

	// 4. Merge + Validate
	merged := Merge(dbRules, projectRules)
	valid, validateErrs := Validate(merged)
	errs = append(errs, validateErrs...)

	// 5. combos(builtin only)
	storedCombos, err := storage.ListCombos(db, storage.DomainDetect)
	if err != nil {
		errs = append(errs, RuleLoadError{Source: "db:detect:combos", Reason: fmt.Sprintf("list combos: %v", err)})
	} else {
		for _, sc := range storedCombos {
			c, convErr := StoredComboToCombo(sc)
			if convErr != nil {
				errs = append(errs, RuleLoadError{Source: "db:detect:combo:" + sc.ID, Reason: convErr.Error()})
				continue
			}
			combos = append(combos, c)
		}
	}
	return valid, combos, errs
}

// LoadInterceptRules 从 sqlite 加载拦截规则(无项目级 Merge——拦截是运行时全局策略)。
func LoadInterceptRules(db *storage.DB) (rules []Rule, errs []RuleLoadError) {
	if db == nil {
		errs = append(errs, RuleLoadError{Source: "db", Reason: "intercept rules db is nil"})
		return nil, errs
	}
	stored, err := storage.ListRulesEnabled(db, storage.DomainIntercept)
	if err != nil {
		errs = append(errs, RuleLoadError{Source: "db:intercept", Reason: fmt.Sprintf("list rules: %v", err)})
		return nil, errs
	}
	for _, s := range stored {
		r, convErr := StoredRuleToRule(s)
		if convErr != nil {
			errs = append(errs, RuleLoadError{Source: "db:intercept:" + s.ID, Reason: convErr.Error()})
			continue
		}
		rules = append(rules, r)
	}
	valid, validateErrs := Validate(rules)
	errs = append(errs, validateErrs...)
	return valid, errs
}

// rulesToStored 把 []Rule 转成 []StoredRule(SyncBuiltin 输入)。
func rulesToStored(rules []Rule, version string) ([]storage.StoredRule, error) {
	out := make([]storage.StoredRule, 0, len(rules))
	for _, r := range rules {
		s, err := RuleToStoredRule(r, "builtin", version)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// combosToStored 把 []ComboRule 转成 []StoredCombo(SyncBuiltin 输入)。
func combosToStored(combos []ComboRule, version string) ([]storage.StoredCombo, error) {
	out := make([]storage.StoredCombo, 0, len(combos))
	for _, c := range combos {
		s, err := ComboToStoredCombo(c, "builtin", version)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// RuleToStoredRule 把 Rule 转成 storage.StoredRule(含 JSON 文本列)。
// 经 RuleToRow 拿到已序列化的 JSON 文本列,再经 RowToMatchJSON 取出(Task 3 偏差 (a):
// RuleRow 直接以 JSON 文本列存储,与 DB 列同构)。
func RuleToStoredRule(r Rule, source, version string) (storage.StoredRule, error) {
	row, err := RuleToRow(r)
	if err != nil {
		return storage.StoredRule{}, err
	}
	matchJSON, pathsJSON, deobJSON, postExcludeJSON, metaJSON, err := RowToMatchJSON(row)
	if err != nil {
		return storage.StoredRule{}, err
	}
	return storage.StoredRule{
		ID: row.ID, Severity: row.Severity, AssetType: row.AssetType,
		MatchJSON: matchJSON, PathsJSON: pathsJSON, Deobfuscation: deobJSON,
		Dotall: row.Dotall, PostExclude: postExcludeJSON, Remediation: row.Remediation,
		Description: row.Description, MetadataJSON: metaJSON,
		Source: source, BuiltinVersion: version,
	}, nil
}

// StoredRuleToRule 把 storage.StoredRule 还原成 Rule(经 RowFromDBColumns + RowToRule)。
// RowFromDBColumns 的参数顺序与 StoredRule 字段一一对应(persist.go:148)。
func StoredRuleToRule(s storage.StoredRule) (Rule, error) {
	row, err := RowFromDBColumns(s.ID, s.Severity, s.AssetType, s.MatchJSON, s.PathsJSON, s.Deobfuscation,
		boolToInt(s.Dotall), s.PostExclude, s.Remediation, s.Description, s.MetadataJSON)
	if err != nil {
		return Rule{}, err
	}
	return RowToRule(row)
}

// ComboToStoredCombo 把 ComboRule 转成 storage.StoredCombo。
//
// 偏差说明(brief 修正 #1):brief 假设 comboRowToJSON(row) 返回
// (requiresJSON, metaJSON, err),但 Task 3 实际落地的 comboRowToJSON 签名是
// (ComboRow) ([]byte, error)——整行 marshal,非按字段拆分。而 ComboToRow 产出的
// ComboRow 已经把 RequiresJSON / MetadataJSON 作为 JSON 文本列直接持有(persist.go:34),
// 故此处直接从 row 取这两个字段,不再调用 comboRowToJSON(那个 helper 仅用于整行序列化场景)。
func ComboToStoredCombo(c ComboRule, source, version string) (storage.StoredCombo, error) {
	row, err := ComboToRow(c)
	if err != nil {
		return storage.StoredCombo{}, err
	}
	return storage.StoredCombo{
		ID: row.ID, Source: source, Severity: row.Severity, Description: row.Description,
		Remediation: row.Remediation, MetadataJSON: row.MetadataJSON, RequiresJSON: row.RequiresJSON,
		BuiltinVersion: version,
	}, nil
}

// StoredComboToCombo 把 storage.StoredCombo 还原成 ComboRule。
// 用 StoredCombo 的 JSON 文本列字段重建 ComboRow,再经 RowToCombo 反序列化
// (与 ComboToStoredCombo 对称,不经 comboRowFromJSON——同理,那是整行反序列化 helper)。
func StoredComboToCombo(s storage.StoredCombo) (ComboRule, error) {
	row := ComboRow{
		ID: s.ID, Severity: s.Severity, Description: s.Description, Remediation: s.Remediation,
		MetadataJSON: s.MetadataJSON, RequiresJSON: s.RequiresJSON,
	}
	return RowToCombo(row)
}

// boolToInt:Go 不允许 bool→int 隐式转换(SQLite dotall 列是 INTEGER)。
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
