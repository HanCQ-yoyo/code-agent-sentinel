// internal/intercept/store.go
package intercept

import (
	"errors"
	"fmt"
	"time"

	"code-agent-sentinel/internal/storage"
)

// ErrNotFound 表示指定 ID 的拦截记录不存在。
var ErrNotFound = errors.New("intercept: record not found")

// Store 把拦截记录持久化到 sqlite intercept_records 表。
type Store struct{ db *storage.DB }

// NewStore 返回指向 db 的 Store。db 为 nil 时所有操作返回错误 (fail-open 空内核可退化)。
func NewStore(db *storage.DB) *Store { return &Store{db: db} }

// Append 写入一条拦截记录。
func (s *Store) Append(rec InterceptRecord) error {
	if s.db == nil {
		return errors.New("intercept: nil db")
	}
	if rec.ID == "" {
		return errors.New("intercept: empty ID")
	}
	row := storage.InterceptRow{
		ID:             rec.ID,
		Timestamp:      rec.Timestamp.Format(time.RFC3339Nano),
		AgentProtocol:  rec.AgentProtocol,
		WorkingDir:     rec.WorkingDir,
		Command:        rec.Command,
		Outcome:        rec.Outcome,
		RuleID:         rec.RuleID,
		PackID:         rec.PackID,
		Severity:       rec.Severity,
		Reason:         rec.Reason,
		EvalDurationUS: rec.EvalDurationUS,
		SessionID:      rec.SessionID,
		ToolName:       rec.ToolName,
		Confidence:     rec.Confidence,
		MatchedSpan:    rec.MatchedSpan,
	}
	return storage.InsertIntercept(s.db, row)
}

// Get 取单条记录。不存在返回 ErrNotFound。
func (s *Store) Get(id string) (*InterceptRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("intercept: nil db")
	}
	row, found, err := storage.GetIntercept(s.db, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNotFound
	}
	ts, err := time.Parse(time.RFC3339Nano, row.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("intercept: parse timestamp %q: %w", row.Timestamp, err)
	}
	return &InterceptRecord{
		ID:             row.ID,
		Timestamp:      ts,
		AgentProtocol:  row.AgentProtocol,
		WorkingDir:     row.WorkingDir,
		Command:        row.Command,
		Outcome:        row.Outcome,
		RuleID:         row.RuleID,
		PackID:         row.PackID,
		Severity:       row.Severity,
		Reason:         row.Reason,
		EvalDurationUS: row.EvalDurationUS,
		SessionID:      row.SessionID,
		ToolName:       row.ToolName,
		Confidence:     row.Confidence,
		MatchedSpan:    row.MatchedSpan,
	}, nil
}

// List 返回所有记录,按 Timestamp 倒序(最新在前)。
func (s *Store) List() ([]InterceptRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("intercept: nil db")
	}
	rows, err := storage.ListIntercepts(s.db)
	if err != nil {
		return nil, err
	}
	out := make([]InterceptRecord, 0, len(rows))
	for _, row := range rows {
		ts, tsErr := time.Parse(time.RFC3339Nano, row.Timestamp)
		if tsErr != nil {
			return nil, fmt.Errorf("intercept: parse timestamp %q: %w", row.Timestamp, tsErr)
		}
		out = append(out, InterceptRecord{
			ID:             row.ID,
			Timestamp:      ts,
			AgentProtocol:  row.AgentProtocol,
			WorkingDir:     row.WorkingDir,
			Command:        row.Command,
			Outcome:        row.Outcome,
			RuleID:         row.RuleID,
			PackID:         row.PackID,
			Severity:       row.Severity,
			Reason:         row.Reason,
			EvalDurationUS: row.EvalDurationUS,
			SessionID:      row.SessionID,
			ToolName:       row.ToolName,
			Confidence:     row.Confidence,
			MatchedSpan:    row.MatchedSpan,
		})
	}
	return out, nil
}

// Delete 删除单条记录。不存在返回 ErrNotFound。
func (s *Store) Delete(id string) error {
	if s.db == nil {
		return fmt.Errorf("intercept: nil db")
	}
	found, err := storage.DeleteIntercept(s.db, id)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	return nil
}
