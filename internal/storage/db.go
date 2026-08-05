// Package storage 提供 sqlite 持久化访问层(纯净包:只依赖标准库 + modernc.org/sqlite,
// 不 import security/configengine)。规则定义/启停覆盖/combo 经此层存取,反序列化在
// ruleengine 侧做。本包只存取 []byte / 字符串字段。
package storage

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // 纯 Go sqlite 驱动,无 CGO
)

// Domain 标识规则所属域:检测(静态扫描)或拦截(运行时 guard)。
type Domain string

const (
	DomainDetect    Domain = "detect"
	DomainIntercept Domain = "intercept"
)

// rulesTable / overridesTable / combosTable 按 domain 返回表名(全小写,SQL 不引号即可用)。
func (d Domain) rulesTable() string {
	return string(d) + "_rules"
}
func (d Domain) overridesTable() string {
	return string(d) + "_overrides"
}
func (d Domain) combosTable() string {
	return string(d) + "_combos"
}

// DB 是 *sql.DB 的薄包装,持有连接池。WAL 模式下多进程读(server + guard)不互相阻塞。
type DB struct {
	sqlDB *sql.DB
}

// Open 打开/创建 db 文件(路径由调用方注入 home 拼出)。开 WAL + busy_timeout。
// 文件不存在则 sql.Open 自动创建;目录须由调用方先 MkdirAll(0o700)。
func Open(path string) (*DB, error) {
	d, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	if err := d.Ping(); err != nil {
		d.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return &DB{sqlDB: d}, nil
}

// Close 关闭连接池。
func (db *DB) Close() error { return db.sqlDB.Close() }

// SQL 暴露底层 *sql.DB(供 repo 文件与测试用)。
func (db *DB) SQL() *sql.DB { return db.sqlDB }

// SchemaInitialized 检查元数据表是否存在(迁移是否已跑过)。
func SchemaInitialized(db *DB) (bool, error) {
	var name string
	err := db.sqlDB.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='schema_meta'").Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// RunMigrations 建表(幂等:CREATE TABLE IF NOT EXISTS)。两域表结构同构。
// rules 表:rule_id 主键,source 区分 builtin/custom,builtin_version 用于升级刷新判定,
//   match_json/paths_json/metadata_json 存 JSON 文本(反序列化在 ruleengine 侧)。
// overrides 表:rule_id 主键,enabled 启停覆盖(缺省=启用,运行时 LEFT JOIN COALESCE)。
//   注意:overrides 不建 FK 到 rules——ListOrphanOverrides 的职责正是检测 rule 被删后
//   留下的孤儿 override(builtin 规则下版本被删,SyncBuiltin 需报告而非静默丢)。
//   若加 FK+CASCADE 则孤儿永远不可能存在;若加 FK 无 CASCADE 则删 rule 行会被 FK 阻止。
//   故 overrides.rule_id 不引用 rules.rule_id;引用完整性由 DeleteRule 在事务内显式
//   清 override 维护,绕过 DeleteRule 的直接 DELETE 才会留孤儿(供审计检测)。
// combos 表:仅 builtin(本次 custom combo 不支持)。
// schema_meta:迁移标记表,存在即表示已迁移。
func RunMigrations(db *DB) error {
	_, err := db.sqlDB.Exec(`
CREATE TABLE IF NOT EXISTS schema_meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
INSERT OR IGNORE INTO schema_meta(key, value) VALUES ('version', '1');

CREATE TABLE IF NOT EXISTS detect_rules (
  rule_id         TEXT PRIMARY KEY,
  source          TEXT NOT NULL,            -- builtin | custom
  severity        TEXT NOT NULL,
  asset_type      TEXT NOT NULL,
  match_json      TEXT NOT NULL,
  paths_json      TEXT,
  deobfuscation   TEXT,                     -- JSON 数组
  dotall          INTEGER NOT NULL DEFAULT 0,
  post_exclude    TEXT,                     -- JSON 数组
  remediation     TEXT,
  description     TEXT,
  metadata_json   TEXT,
  builtin_version TEXT,                     -- builtin 行=引擎版本;custom 行=NULL
  updated_at      TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS detect_overrides (
  rule_id    TEXT PRIMARY KEY,
  enabled    INTEGER NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS detect_combos (
  rule_id         TEXT PRIMARY KEY,
  source          TEXT NOT NULL,
  severity        TEXT NOT NULL,
  description     TEXT,
  remediation     TEXT,
  metadata_json   TEXT,
  requires_json   TEXT NOT NULL,
  builtin_version TEXT,
  updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS intercept_rules (
  rule_id         TEXT PRIMARY KEY,
  source          TEXT NOT NULL,
  severity        TEXT NOT NULL,
  asset_type      TEXT NOT NULL,
  match_json      TEXT NOT NULL,
  paths_json      TEXT,
  deobfuscation   TEXT,
  dotall          INTEGER NOT NULL DEFAULT 0,
  post_exclude    TEXT,
  remediation     TEXT,
  description     TEXT,
  metadata_json   TEXT,
  builtin_version TEXT,
  updated_at      TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS intercept_overrides (
  rule_id    TEXT PRIMARY KEY,
  enabled    INTEGER NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS intercept_combos (
  rule_id         TEXT PRIMARY KEY,
  source          TEXT NOT NULL,
  severity        TEXT NOT NULL,
  description     TEXT,
  remediation     TEXT,
  metadata_json   TEXT,
  requires_json   TEXT NOT NULL,
  builtin_version TEXT,
  updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS scan_history (
  id              TEXT PRIMARY KEY,
  agent_id        TEXT NOT NULL,
  batch_id        TEXT,
  started_at      TEXT NOT NULL,
  duration_ns     INTEGER NOT NULL,
  scope           TEXT NOT NULL DEFAULT 'global',
  scope_path      TEXT,
  finding_count   INTEGER NOT NULL DEFAULT 0,
  health_score    INTEGER,
  health_band     TEXT,
  detector_avail  INTEGER NOT NULL DEFAULT 0,
  detector_total  INTEGER NOT NULL DEFAULT 0,
  findings_json   TEXT NOT NULL,
  detectors_json  TEXT NOT NULL,
  inventory_json  TEXT,
  projects_json   TEXT
);
CREATE INDEX IF NOT EXISTS idx_scan_history_agent_started ON scan_history(agent_id, started_at DESC);

CREATE TABLE IF NOT EXISTS intercept_records (
  id              TEXT PRIMARY KEY,
  timestamp       TEXT NOT NULL,
  agent_protocol  TEXT NOT NULL DEFAULT 'claude',
  working_dir     TEXT,
  command         TEXT NOT NULL,
  outcome         TEXT NOT NULL,
  rule_id         TEXT,
  pack_id         TEXT,
  severity        TEXT,
  reason          TEXT,
  eval_duration_us INTEGER NOT NULL DEFAULT 0,
  session_id      TEXT,
  tool_name       TEXT,
  confidence      TEXT,
  matched_span    TEXT
);
CREATE INDEX IF NOT EXISTS idx_intercept_timestamp ON intercept_records(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_intercept_tool_name ON intercept_records(tool_name);
CREATE INDEX IF NOT EXISTS idx_intercept_outcome ON intercept_records(outcome);

CREATE TABLE IF NOT EXISTS finding_states (
  fingerprint TEXT PRIMARY KEY,
  status      TEXT NOT NULL,
  priority    TEXT,
  note        TEXT,
  source      TEXT,
  updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS allowlist_entries (
  command    TEXT PRIMARY KEY,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS user_prefs (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS schedules (
  agent_id   TEXT PRIMARY KEY,
  enabled    INTEGER NOT NULL DEFAULT 1,
  interval   TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL
);
`)
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

// GetUserPref 读取 user_prefs 表中 key 对应的 value。不存在返回空串。
func GetUserPref(db *DB, key string) (string, error) {
	var v string
	err := db.sqlDB.QueryRow("SELECT value FROM user_prefs WHERE key = ?", key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get user_pref %s: %w", key, err)
	}
	return v, nil
}

// SetUserPref INSERT OR REPLACE user_prefs。key 不存在时插入,存在时更新。
func SetUserPref(db *DB, key, value string) error {
	_, err := db.sqlDB.Exec(
		"INSERT OR REPLACE INTO user_prefs(key, value) VALUES (?, ?)",
		key, value)
	if err != nil {
		return fmt.Errorf("set user_pref %s: %w", key, err)
	}
	return nil
}

// ScheduleRow 对应 schedules 表一行。
type ScheduleRow struct {
	AgentID   string
	Enabled   bool
	Interval  string
	UpdatedAt string
}

// ListSchedules 返回 schedules 表全部行。
func ListSchedules(db *DB) ([]ScheduleRow, error) {
	rows, err := db.sqlDB.Query("SELECT agent_id, enabled, interval, updated_at FROM schedules ORDER BY agent_id")
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	defer rows.Close()
	var out []ScheduleRow
	for rows.Next() {
		var r ScheduleRow
		var enabledInt int
		if err := rows.Scan(&r.AgentID, &enabledInt, &r.Interval, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan schedule: %w", err)
		}
		r.Enabled = enabledInt != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpsertSchedule 插入或更新 schedules 表一行。
func UpsertSchedule(db *DB, agentID string, enabled int, interval, updatedAt string) error {
	_, err := db.sqlDB.Exec(
		`INSERT OR REPLACE INTO schedules(agent_id, enabled, interval, updated_at) VALUES (?, ?, ?, ?)`,
		agentID, enabled, interval, updatedAt)
	if err != nil {
		return fmt.Errorf("upsert schedule %s: %w", agentID, err)
	}
	return nil
}

// DeleteSchedule 删除 schedules 表一行。
func DeleteSchedule(db *DB, agentID string) error {
	_, err := db.sqlDB.Exec("DELETE FROM schedules WHERE agent_id = ?", agentID)
	if err != nil {
		return fmt.Errorf("delete schedule %s: %w", agentID, err)
	}
	return nil
}
