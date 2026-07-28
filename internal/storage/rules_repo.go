package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// StoredRule 是存于 *_rules 表的一行(纯字段 + JSON 文本列)。
// storage 不 import ruleengine,故独立定义;ruleengine.RuleRow 经 load_db.go 适配转换。
// Source 字段来自 UpsertRule 参数(不在此结构),但 GetRule/ListRules 会回填。
type StoredRule struct {
	ID             string
	Source         string // builtin | custom
	Severity       string
	AssetType      string
	MatchJSON      string
	PathsJSON      string
	Deobfuscation  string // JSON 数组文本,空为 ""
	Dotall         bool
	PostExclude    string // JSON 数组文本
	Remediation    string
	Description    string
	MetadataJSON   string
	BuiltinVersion string // custom 行为 ""
	UpdatedAt      string
}

// nowStamp 返回当前时间的 RFC3339 文本(供 updated_at)。
// 注意:storage 测试与生产都用真实时间(本包无业务不变量依赖时间单调)。
func nowStamp() string { return time.Now().UTC().Format(time.RFC3339) }

// UpsertRule 插入或覆盖一条规则(按 rule_id 主键)。source 决定 builtin/custom;
// builtinVersion 仅 builtin 行有意义(custom 传 "")。
// 注意:domain 仅来自本包导出常量 DomainDetect/DomainIntercept,不接受外部输入拼接,
// 故 domain.rulesTable() 的字符串拼接安全(见 Task 1 review 注记)。
func UpsertRule(db *DB, domain Domain, source string, r StoredRule, builtinVersion string) error {
	tbl := domain.rulesTable()
	_, err := db.sqlDB.Exec(
		`INSERT INTO `+tbl+` (rule_id, source, severity, asset_type, match_json, paths_json,
		   deobfuscation, dotall, post_exclude, remediation, description, metadata_json,
		   builtin_version, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(rule_id) DO UPDATE SET
		   source=excluded.source, severity=excluded.severity, asset_type=excluded.asset_type,
		   match_json=excluded.match_json, paths_json=excluded.paths_json,
		   deobfuscation=excluded.deobfuscation, dotall=excluded.dotall,
		   post_exclude=excluded.post_exclude, remediation=excluded.remediation,
		   description=excluded.description, metadata_json=excluded.metadata_json,
		   builtin_version=excluded.builtin_version, updated_at=excluded.updated_at`,
		r.ID, source, r.Severity, r.AssetType, r.MatchJSON, r.PathsJSON,
		r.Deobfuscation, r.Dotall, r.PostExclude, r.Remediation, r.Description, r.MetadataJSON,
		builtinVersionOrNull(source, builtinVersion), nowStamp())
	if err != nil {
		return fmt.Errorf("upsert rule %s: %w", r.ID, err)
	}
	return nil
}

// builtinVersionOrNull:custom 行 builtin_version 存 NULL;builtin 行存版本号。
func builtinVersionOrNull(source, bv string) any {
	if source == "builtin" && bv != "" {
		return bv
	}
	return nil
}

// scanRule 从 sql.Row/Rows 扫描出 StoredRule(含 source/builtin_version)。
// prefixCols 保留参数(当前未使用,为未来列扩展预留);调用方统一传 false。
func scanRule(scanner interface{ Scan(...any) error }, prefixCols bool) (StoredRule, error) {
	var r StoredRule
	var bv sql.NullString
	err := scanner.Scan(&r.ID, &r.Source, &r.Severity, &r.AssetType, &r.MatchJSON, &r.PathsJSON,
		&r.Deobfuscation, &r.Dotall, &r.PostExclude, &r.Remediation, &r.Description, &r.MetadataJSON,
		&bv, &r.UpdatedAt)
	r.BuiltinVersion = bv.String
	return r, err
}

// GetRule 取单条。不存在返回 (zero, false, nil)。
func GetRule(db *DB, domain Domain, ruleID string) (StoredRule, bool, error) {
	row := db.sqlDB.QueryRow(
		`SELECT rule_id, source, severity, asset_type, match_json, paths_json,
		        deobfuscation, dotall, post_exclude, remediation, description, metadata_json,
		        builtin_version, updated_at
		 FROM `+domain.rulesTable()+` WHERE rule_id=?`, ruleID)
	r, err := scanRule(row, false)
	if err == sql.ErrNoRows {
		return StoredRule{}, false, nil
	}
	if err != nil {
		return StoredRule{}, false, fmt.Errorf("get rule %s: %w", ruleID, err)
	}
	return r, true, nil
}

// ListRules 列出全部规则(不含 enabled;enabled 由 ListRulesWithEnabled 或调用方 JOIN)。
func ListRules(db *DB, domain Domain) ([]StoredRule, error) {
	rows, err := db.sqlDB.Query(
		`SELECT rule_id, source, severity, asset_type, match_json, paths_json,
		        deobfuscation, dotall, post_exclude, remediation, description, metadata_json,
		        builtin_version, updated_at
		 FROM `+domain.rulesTable()+` ORDER BY rule_id`)
	if err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}
	defer rows.Close()
	var out []StoredRule
	for rows.Next() {
		r, err := scanRule(rows, false)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteRule 删除一条规则。由于 schema 不使用 ON DELETE CASCADE(见 db.go 注释),
// 此处在事务内显式清对应 override,效果等价于 CASCADE。
// 注意:不应允许删 builtin 行(引擎 SyncBuiltin 会重建);调用方校验 source==custom。
// 事务保证:override 清理与 rule 删除原子完成,不会留下悬挂 override(除非调用方
// 直接 DELETE FROM rules 绕过本函数——那正是 ListOrphanOverrides 要检测的场景)。
func DeleteRule(db *DB, domain Domain, ruleID string) error {
	tx, err := db.sqlDB.Begin()
	if err != nil {
		return fmt.Errorf("delete rule %s: begin tx: %w", ruleID, err)
	}
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()
	if _, err := tx.Exec(`DELETE FROM `+domain.overridesTable()+` WHERE rule_id=?`, ruleID); err != nil {
		return fmt.Errorf("delete rule %s: clear override: %w", ruleID, err)
	}
	if _, err := tx.Exec(`DELETE FROM `+domain.rulesTable()+` WHERE rule_id=?`, ruleID); err != nil {
		return fmt.Errorf("delete rule %s: %w", ruleID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete rule %s: commit: %w", ruleID, err)
	}
	tx = nil
	return nil
}

// SetOverride upsert 启停覆盖。enabled=false 表示禁用。
func SetOverride(db *DB, domain Domain, ruleID string, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := db.sqlDB.Exec(
		`INSERT INTO `+domain.overridesTable()+` (rule_id, enabled, updated_at)
		 VALUES (?,?,?)
		 ON CONFLICT(rule_id) DO UPDATE SET enabled=excluded.enabled, updated_at=excluded.updated_at`,
		ruleID, enabledInt, nowStamp())
	if err != nil {
		return fmt.Errorf("set override %s: %w", ruleID, err)
	}
	return nil
}

// ClearOverride 删除启停覆盖(恢复默认=启用)。
func ClearOverride(db *DB, domain Domain, ruleID string) error {
	_, err := db.sqlDB.Exec(`DELETE FROM `+domain.overridesTable()+` WHERE rule_id=?`, ruleID)
	if err != nil {
		return fmt.Errorf("clear override %s: %w", ruleID, err)
	}
	return nil
}

// GetOverride 取启停状态。exists=false 表示无 override(=默认启用)。
func GetOverride(db *DB, domain Domain, ruleID string) (enabled bool, exists bool, err error) {
	var en int
	err = db.sqlDB.QueryRow(
		`SELECT enabled FROM `+domain.overridesTable()+` WHERE rule_id=?`, ruleID).Scan(&en)
	if err == sql.ErrNoRows {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("get override %s: %w", ruleID, err)
	}
	return en != 0, true, nil
}

// ListOrphanOverrides 列出 overrides 表中 rule_id 在 rules 表已不存在的孤儿
// (builtin 规则下版本被删后 SyncBuiltin 检测用,报告不自动删)。
func ListOrphanOverrides(db *DB, domain Domain) ([]string, error) {
	rows, err := db.sqlDB.Query(
		`SELECT o.rule_id FROM `+domain.overridesTable()+` o
		 LEFT JOIN `+domain.rulesTable()+` r ON o.rule_id = r.rule_id
		 WHERE r.rule_id IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("list orphan overrides: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
