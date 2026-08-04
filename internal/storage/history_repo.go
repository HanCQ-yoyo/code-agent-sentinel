package storage

import (
	"database/sql"
	"errors"
	"fmt"
)

// HistoryRow 是 scan_history 表的完整一行。
type HistoryRow struct {
	ID, AgentID, BatchID, StartedAt, Scope, ScopePath          string
	DurationNs                                                  int64
	FindingCount, HealthScore, DetectorAvail, DetectorTotal     int
	HealthBand                                                  string
	FindingsJSON, DetectorsJSON, InventoryJSON, ProjectsJSON    []byte
}

// HistorySummaryRow 是查询列表用的摘要行(不含 JSON 大字段)。
type HistorySummaryRow struct {
	ID, AgentID, BatchID, StartedAt, Scope, ScopePath          string
	FindingCount, HealthScore, DetectorAvail, DetectorTotal     int
	HealthBand                                                  string
}

// InsertHistory 插入一条完整扫描记录。
func InsertHistory(db *DB, r HistoryRow) error {
	_, err := db.sqlDB.Exec(
		`INSERT INTO scan_history (id, agent_id, batch_id, started_at, duration_ns,
		 scope, scope_path, finding_count, health_score, health_band,
		 detector_avail, detector_total, findings_json, detectors_json,
		 inventory_json, projects_json)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.AgentID, r.BatchID, r.StartedAt, r.DurationNs,
		r.Scope, r.ScopePath, r.FindingCount, r.HealthScore, r.HealthBand,
		r.DetectorAvail, r.DetectorTotal, r.FindingsJSON, r.DetectorsJSON,
		r.InventoryJSON, r.ProjectsJSON)
	if err != nil {
		return fmt.Errorf("insert scan_history %s: %w", r.ID, err)
	}
	return nil
}

// GetHistory 取单条完整记录(含 JSON 列)。不存在返回 (zero, false, nil)。
func GetHistory(db *DB, id string) (HistoryRow, bool, error) {
	var r HistoryRow
	err := db.sqlDB.QueryRow(
		`SELECT id, agent_id, batch_id, started_at, duration_ns,
		        scope, scope_path, finding_count, health_score, health_band,
		        detector_avail, detector_total, findings_json, detectors_json,
		        inventory_json, projects_json
		 FROM scan_history WHERE id=?`, id).
		Scan(&r.ID, &r.AgentID, &r.BatchID, &r.StartedAt, &r.DurationNs,
			&r.Scope, &r.ScopePath, &r.FindingCount, &r.HealthScore, &r.HealthBand,
			&r.DetectorAvail, &r.DetectorTotal, &r.FindingsJSON, &r.DetectorsJSON,
			&r.InventoryJSON, &r.ProjectsJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HistoryRow{}, false, nil
		}
		return HistoryRow{}, false, fmt.Errorf("get scan_history %s: %w", id, err)
	}
	return r, true, nil
}

// ListHistorySummaries 返回摘要列表(不含 JSON 大字段),按 started_at 倒序。
func ListHistorySummaries(db *DB) ([]HistorySummaryRow, error) {
	rows, err := db.sqlDB.Query(
		`SELECT id, agent_id, batch_id, started_at, scope, scope_path,
		        finding_count, health_score, health_band, detector_avail, detector_total
		 FROM scan_history ORDER BY started_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list scan_history: %w", err)
	}
	defer rows.Close()
	var out []HistorySummaryRow
	for rows.Next() {
		var s HistorySummaryRow
		if err := rows.Scan(&s.ID, &s.AgentID, &s.BatchID, &s.StartedAt, &s.Scope, &s.ScopePath,
			&s.FindingCount, &s.HealthScore, &s.HealthBand, &s.DetectorAvail, &s.DetectorTotal); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DeleteHistory 删除单条扫描记录。不存在返回 false。
func DeleteHistory(db *DB, id string) (bool, error) {
	res, err := db.sqlDB.Exec(`DELETE FROM scan_history WHERE id=?`, id)
	if err != nil {
		return false, fmt.Errorf("delete scan_history %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
