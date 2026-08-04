package storage

import (
	"database/sql"
	"errors"
	"fmt"
)

// InterceptRow 是 intercept_records 表的一行。
type InterceptRow struct {
	ID, Timestamp, AgentProtocol, WorkingDir, Command, Outcome string
	RuleID, PackID, Severity, Reason, SessionID, ToolName       string
	Confidence, MatchedSpan                                     string
	EvalDurationUS                                              int64
}

// InsertIntercept 插入一条拦截记录。
func InsertIntercept(db *DB, r InterceptRow) error {
	_, err := db.sqlDB.Exec(
		`INSERT INTO intercept_records (id, timestamp, agent_protocol, working_dir, command,
		 outcome, rule_id, pack_id, severity, reason, eval_duration_us, session_id,
		 tool_name, confidence, matched_span)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.Timestamp, r.AgentProtocol, r.WorkingDir, r.Command, r.Outcome,
		r.RuleID, r.PackID, r.Severity, r.Reason, r.EvalDurationUS, r.SessionID,
		r.ToolName, r.Confidence, r.MatchedSpan)
	if err != nil {
		return fmt.Errorf("insert intercept %s: %w", r.ID, err)
	}
	return nil
}

// GetIntercept 取单条拦截记录。不存在返回 (zero, false, nil)。
func GetIntercept(db *DB, id string) (InterceptRow, bool, error) {
	var r InterceptRow
	err := db.sqlDB.QueryRow(
		`SELECT id, timestamp, agent_protocol, working_dir, command, outcome,
		        rule_id, pack_id, severity, reason, eval_duration_us, session_id,
		        tool_name, confidence, matched_span
		 FROM intercept_records WHERE id=?`, id).
		Scan(&r.ID, &r.Timestamp, &r.AgentProtocol, &r.WorkingDir, &r.Command, &r.Outcome,
			&r.RuleID, &r.PackID, &r.Severity, &r.Reason, &r.EvalDurationUS, &r.SessionID,
			&r.ToolName, &r.Confidence, &r.MatchedSpan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return InterceptRow{}, false, nil
		}
		return InterceptRow{}, false, fmt.Errorf("get intercept %s: %w", id, err)
	}
	return r, true, nil
}

// ListIntercepts 返回全部拦截记录,按 timestamp 倒序。
func ListIntercepts(db *DB) ([]InterceptRow, error) {
	rows, err := db.sqlDB.Query(
		`SELECT id, timestamp, agent_protocol, working_dir, command, outcome,
		        rule_id, pack_id, severity, reason, eval_duration_us, session_id,
		        tool_name, confidence, matched_span
		 FROM intercept_records ORDER BY timestamp DESC`)
	if err != nil {
		return nil, fmt.Errorf("list intercepts: %w", err)
	}
	defer rows.Close()
	var out []InterceptRow
	for rows.Next() {
		var r InterceptRow
		if err := rows.Scan(&r.ID, &r.Timestamp, &r.AgentProtocol, &r.WorkingDir, &r.Command,
			&r.Outcome, &r.RuleID, &r.PackID, &r.Severity, &r.Reason, &r.EvalDurationUS,
			&r.SessionID, &r.ToolName, &r.Confidence, &r.MatchedSpan); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteIntercept 删除单条拦截记录。不存在返回 false。
func DeleteIntercept(db *DB, id string) (bool, error) {
	res, err := db.sqlDB.Exec(`DELETE FROM intercept_records WHERE id=?`, id)
	if err != nil {
		return false, fmt.Errorf("delete intercept %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
