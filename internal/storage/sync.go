package storage

import (
	"database/sql"
	"fmt"
)

// SyncBuiltinResult 是 SyncBuiltin 的返回:刷新了多少 builtin 行 + 哪些 override 成了孤儿。
type SyncBuiltinResult struct {
	Refreshed       int
	OrphanOverrides []string
}

// StoredCombo 是存于 *_combos 表的一行(仅 builtin)。requires_json 是 JSON 文本。
// Task 6 的 combosToStored 会构造这些,故保持导出。
type StoredCombo struct {
	ID             string
	Source         string
	Severity       string
	Description    string
	Remediation    string
	MetadataJSON   string
	RequiresJSON   string
	BuiltinVersion string
}

// SyncBuiltin 把 embed 内置规则同步进 db:
//  1. UPSERT 所有传入的 builtin 行(source=builtin,整行覆盖含 builtin_version)。
//  2. 删除 db 里 source=builtin 但不在传入列表中的行(下版本删的规则)。
//  3. 不碰 source=custom 行、不碰 *_overrides(用户态保留)。
//  4. 报告孤儿 override(规则已删但 override 还在)——不自动删,防误删用户启停。
//  5. combos 同理(本次仅 builtin)。
//
// 幂等:同 version 重复调用安全。单事务保证一致性。
//
// 注意:本函数删除 stale builtin 行走直接 DELETE FROM <rules> WHERE source='builtin' ...,
// 而非 DeleteRule——故意如此。DeleteRule 会事务内清 override(等价 CASCADE),但
// SyncBuiltin 要让被删 builtin 规则的 override 成为孤儿并报告(供审计/用户决策),
// 故必须绕过 DeleteRule。参见 db.go 关于 overrides 不建 FK 的注释。
func SyncBuiltin(db *DB, domain Domain, rules []StoredRule, combos []StoredCombo, version string) (SyncBuiltinResult, error) {
	tx, err := db.sqlDB.Begin()
	if err != nil {
		return SyncBuiltinResult{}, fmt.Errorf("sync begin tx: %w", err)
	}
	defer tx.Rollback() // 已 commit 则 no-op

	rulesTbl := domain.rulesTable()
	combosTbl := domain.combosTable()

	// 1. UPSERT builtin rules
	for _, r := range rules {
		if _, err := tx.Exec(
			`INSERT INTO `+rulesTbl+` (rule_id, source, severity, asset_type, match_json, paths_json,
			   deobfuscation, dotall, post_exclude, remediation, description, metadata_json,
			   builtin_version, updated_at)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			 ON CONFLICT(rule_id) DO UPDATE SET
			   source='builtin', severity=excluded.severity, asset_type=excluded.asset_type,
			   match_json=excluded.match_json, paths_json=excluded.paths_json,
			   deobfuscation=excluded.deobfuscation, dotall=excluded.dotall,
			   post_exclude=excluded.post_exclude, remediation=excluded.remediation,
			   description=excluded.description, metadata_json=excluded.metadata_json,
			   builtin_version=?, updated_at=excluded.updated_at`,
			r.ID, "builtin", r.Severity, r.AssetType, r.MatchJSON, r.PathsJSON,
			r.Deobfuscation, r.Dotall, r.PostExclude, r.Remediation, r.Description, r.MetadataJSON,
			version, nowStamp(), version); err != nil {
			return SyncBuiltinResult{}, fmt.Errorf("sync upsert rule %s: %w", r.ID, err)
		}
	}

	// 2. 删除 db 里 source=builtin 但不在传入列表的行。
	//    用临时表比拼 IN 子句更稳(规则数可能上百)。建临时表 → 插入传入 id → 反向删除。
	if err := deleteStaleBuiltin(tx, rulesTbl, rules); err != nil {
		return SyncBuiltinResult{}, err
	}

	// 3. combos(同模式,仅 builtin,无 custom)
	for _, c := range combos {
		if _, err := tx.Exec(
			`INSERT INTO `+combosTbl+` (rule_id, source, severity, description, remediation,
			   metadata_json, requires_json, builtin_version, updated_at)
			 VALUES (?,?,?,?,?,?,?,?,?)
			 ON CONFLICT(rule_id) DO UPDATE SET
			   source='builtin', severity=excluded.severity, description=excluded.description,
			   remediation=excluded.remediation, metadata_json=excluded.metadata_json,
			   requires_json=excluded.requires_json, builtin_version=?, updated_at=excluded.updated_at`,
			c.ID, "builtin", c.Severity, c.Description, c.Remediation, c.MetadataJSON,
			c.RequiresJSON, version, nowStamp(), version); err != nil {
			return SyncBuiltinResult{}, fmt.Errorf("sync upsert combo %s: %w", c.ID, err)
		}
	}
	if err := deleteStaleBuiltinCombos(tx, combosTbl, combos); err != nil {
		return SyncBuiltinResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return SyncBuiltinResult{}, fmt.Errorf("sync commit: %w", err)
	}

	// 4. 报告孤儿 override(规则行已删但 override 还在)
	orphans, err := ListOrphanOverrides(db, domain)
	if err != nil {
		return SyncBuiltinResult{Refreshed: len(rules)}, err
	}
	return SyncBuiltinResult{Refreshed: len(rules), OrphanOverrides: orphans}, nil
}

// deleteStaleBuiltin 删除 rules 表里 source=builtin 且 id 不在 kept 中的行。
// 临时表 _sync_keep 用 CREATE TEMP TABLE IF NOT EXISTS + DELETE + 重填,使函数可重入。
// 仅删 source='builtin' 行;custom 行用户所有,引擎升级不应删。
func deleteStaleBuiltin(tx *sql.Tx, tbl string, kept []StoredRule) error {
	// SQLite 不支持参数化 IN(几千项)。用临时表做反连接更稳。
	if _, err := tx.Exec(`CREATE TEMP TABLE IF NOT EXISTS _sync_keep(id TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create temp keep table: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM _sync_keep`); err != nil {
		return fmt.Errorf("clear temp keep table: %w", err)
	}
	for _, r := range kept {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO _sync_keep(id) VALUES (?)`, r.ID); err != nil {
			return fmt.Errorf("populate temp keep table: %w", err)
		}
	}
	if _, err := tx.Exec(
		`DELETE FROM `+tbl+` WHERE source='builtin' AND rule_id NOT IN (SELECT id FROM _sync_keep)`); err != nil {
		return fmt.Errorf("delete stale builtin rules: %w", err)
	}
	return nil
}

func deleteStaleBuiltinCombos(tx *sql.Tx, tbl string, kept []StoredCombo) error {
	if _, err := tx.Exec(`CREATE TEMP TABLE IF NOT EXISTS _sync_keep_combo(id TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create temp keep combo table: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM _sync_keep_combo`); err != nil {
		return fmt.Errorf("clear temp keep combo table: %w", err)
	}
	for _, c := range kept {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO _sync_keep_combo(id) VALUES (?)`, c.ID); err != nil {
			return fmt.Errorf("populate temp keep combo table: %w", err)
		}
	}
	if _, err := tx.Exec(
		`DELETE FROM `+tbl+` WHERE source='builtin' AND rule_id NOT IN (SELECT id FROM _sync_keep_combo)`); err != nil {
		return fmt.Errorf("delete stale builtin combos: %w", err)
	}
	return nil
}
