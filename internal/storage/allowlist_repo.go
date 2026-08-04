package storage

import (
	"fmt"
	"time"
)

// AllowlistEntryRow 是 allowlist_entries 表的一行。
type AllowlistEntryRow struct {
	Command   string
	CreatedAt string
}

// ReplaceAllowlist 全量替换放行清单(事务内先 DELETE ALL 再 INSERT 新列表)。
func ReplaceAllowlist(db *DB, entries []AllowlistEntryRow) error {
	tx, err := db.sqlDB.Begin()
	if err != nil {
		return fmt.Errorf("replace allowlist: begin tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM allowlist_entries`); err != nil {
		return fmt.Errorf("replace allowlist: delete all: %w", err)
	}
	for _, e := range entries {
		ts := e.CreatedAt
		if ts == "" {
			ts = time.Now().UTC().Format(time.RFC3339)
		}
		if _, err := tx.Exec(
			`INSERT INTO allowlist_entries (command, created_at) VALUES (?, ?)`,
			e.Command, ts); err != nil {
			return fmt.Errorf("replace allowlist: insert %q: %w", e.Command, err)
		}
	}
	return tx.Commit()
}

// ListAllowlist 返回全部放行清单条目。
func ListAllowlist(db *DB) ([]AllowlistEntryRow, error) {
	rows, err := db.sqlDB.Query(`SELECT command, created_at FROM allowlist_entries ORDER BY command`)
	if err != nil {
		return nil, fmt.Errorf("list allowlist: %w", err)
	}
	defer rows.Close()
	var out []AllowlistEntryRow
	for rows.Next() {
		var r AllowlistEntryRow
		if err := rows.Scan(&r.Command, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
