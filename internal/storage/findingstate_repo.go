package storage

import (
	"database/sql"
	"fmt"
)

// FindingStateRow 是 finding_states 表的一行。
type FindingStateRow struct {
	Fingerprint string
	Status      string
	Priority    string
	Note        string
	Source      string
	UpdatedAt   string
}

// UpsertFindingState 插入或更新处置状态(按 fingerprint 主键)。
func UpsertFindingState(db *DB, fp, status, priority, note, source, updatedAt string) error {
	_, err := db.sqlDB.Exec(
		`INSERT INTO finding_states (fingerprint, status, priority, note, source, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(fingerprint) DO UPDATE SET
		   status=excluded.status, priority=excluded.priority, note=excluded.note,
		   source=excluded.source, updated_at=excluded.updated_at`,
		fp, status, priority, note, source, updatedAt)
	if err != nil {
		return fmt.Errorf("upsert finding_state %s: %w", fp, err)
	}
	return nil
}

// GetFindingState 取单条处置状态。不存在返回 (zero, false, nil)。
func GetFindingState(db *DB, fp string) (FindingStateRow, bool, error) {
	var r FindingStateRow
	err := db.sqlDB.QueryRow(
		`SELECT fingerprint, status, priority, note, source, updated_at
		 FROM finding_states WHERE fingerprint=?`, fp).
		Scan(&r.Fingerprint, &r.Status, &r.Priority, &r.Note, &r.Source, &r.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return FindingStateRow{}, false, nil
		}
		return FindingStateRow{}, false, fmt.Errorf("get finding_state %s: %w", fp, err)
	}
	return r, true, nil
}

// ListFindingStates 返回全部处置状态。
func ListFindingStates(db *DB) ([]FindingStateRow, error) {
	rows, err := db.sqlDB.Query(
		`SELECT fingerprint, status, priority, note, source, updated_at
		 FROM finding_states ORDER BY fingerprint`)
	if err != nil {
		return nil, fmt.Errorf("list finding_states: %w", err)
	}
	defer rows.Close()
	var out []FindingStateRow
	for rows.Next() {
		var r FindingStateRow
		if err := rows.Scan(&r.Fingerprint, &r.Status, &r.Priority, &r.Note, &r.Source, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteFindingState 删除单条处置状态。不存在返回 false。
func DeleteFindingState(db *DB, fp string) (bool, error) {
	res, err := db.sqlDB.Exec(`DELETE FROM finding_states WHERE fingerprint=?`, fp)
	if err != nil {
		return false, fmt.Errorf("delete finding_state %s: %w", fp, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
