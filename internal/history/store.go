// internal/history/store.go
package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security"
	"code-agent-sentinel/internal/storage"
)

// ErrNotFound 表示指定 ID 的记录不存在。
var ErrNotFound = errors.New("history: record not found")

// Store 把扫描记录持久化到 sqlite scan_history 表。
type Store struct{ db *storage.DB }

// NewStore 返回指向 db 的 Store。db 为 nil 时所有操作返回错误。
func NewStore(db *storage.DB) *Store { return &Store{db: db} }

// Save 写入一条扫描记录。
func (s *Store) Save(rec ScanRecord) error {
	if s.db == nil {
		return errors.New("history: nil db")
	}
	if rec.ID == "" {
		return errors.New("history: empty ID")
	}
	findingsJSON, err := json.Marshal(rec.Findings)
	if err != nil {
		return fmt.Errorf("history: marshal findings: %w", err)
	}
	detectorsJSON, err := json.Marshal(rec.Detectors)
	if err != nil {
		return fmt.Errorf("history: marshal detectors: %w", err)
	}
	var inventoryJSON []byte
	if rec.Inventory != nil {
		inventoryJSON, err = json.Marshal(rec.Inventory)
		if err != nil {
			return fmt.Errorf("history: marshal inventory: %w", err)
		}
	}
	var projectsJSON []byte
	if rec.Projects != nil && len(rec.Projects) > 0 {
		projectsJSON, err = json.Marshal(rec.Projects)
		if err != nil {
			return fmt.Errorf("history: marshal projects: %w", err)
		}
	}
	var findingCount int
	if rec.Findings != nil {
		findingCount = len(rec.Findings)
	}
	var healthScore int
	var healthBand string
	if rec.HealthScore != nil {
		healthScore = rec.HealthScore.Score
		healthBand = rec.HealthScore.Band
	}
	var avail, total int
	if rec.Detectors != nil {
		total = len(rec.Detectors)
		for _, d := range rec.Detectors {
			if d.Available {
				avail++
			}
		}
	}
	scope := rec.Scope
	if scope == "" {
		scope = "global"
	}
	row := storage.HistoryRow{
		ID:             rec.ID,
		AgentID:        rec.AgentID,
		BatchID:        rec.BatchID,
		StartedAt:      rec.StartedAt.Format(time.RFC3339Nano),
		DurationNs:     int64(rec.Duration),
		Scope:          scope,
		ScopePath:      rec.ScopePath,
		FindingCount:   findingCount,
		HealthScore:    healthScore,
		HealthBand:     healthBand,
		DetectorAvail:  avail,
		DetectorTotal:  total,
		FindingsJSON:   findingsJSON,
		DetectorsJSON:  detectorsJSON,
		InventoryJSON:  inventoryJSON,
		ProjectsJSON:   projectsJSON,
	}
	return storage.InsertHistory(s.db, row)
}

// Get 取单条完整记录。不存在返回 ErrNotFound。
func (s *Store) Get(id string) (*ScanRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("history: nil db")
	}
	row, found, err := storage.GetHistory(s.db, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNotFound
	}
	return rowToScanRecord(row)
}

// rowToScanRecord 将 HistoryRow 反序列化为 ScanRecord。
func rowToScanRecord(row storage.HistoryRow) (*ScanRecord, error) {
	ts, err := time.Parse(time.RFC3339Nano, row.StartedAt)
	if err != nil {
		return nil, fmt.Errorf("history: parse started_at %q: %w", row.StartedAt, err)
	}
	rec := &ScanRecord{
		ID:        row.ID,
		AgentID:   row.AgentID,
		BatchID:   row.BatchID,
		StartedAt: ts,
		Duration:  time.Duration(row.DurationNs),
		Scope:     row.Scope,
		ScopePath: row.ScopePath,
	}
	if rec.Scope == "global" {
		rec.Scope = "" // 保持向后兼容:旧记录 scope="" 视为 global,读回时归一化为空。
	}
	if row.HealthScore != 0 || row.HealthBand != "" {
		rec.HealthScore = &security.HealthScore{Score: row.HealthScore, Band: row.HealthBand}
	}
	if len(row.FindingsJSON) > 0 {
		if err := json.Unmarshal(row.FindingsJSON, &rec.Findings); err != nil {
			return nil, fmt.Errorf("history: unmarshal findings: %w", err)
		}
	}
	if len(row.DetectorsJSON) > 0 {
		if err := json.Unmarshal(row.DetectorsJSON, &rec.Detectors); err != nil {
			return nil, fmt.Errorf("history: unmarshal detectors: %w", err)
		}
	}
	if len(row.InventoryJSON) > 0 {
		var inv configengine.Inventory
		if err := json.Unmarshal(row.InventoryJSON, &inv); err != nil {
			return nil, fmt.Errorf("history: unmarshal inventory: %w", err)
		}
		rec.Inventory = &inv
	}
	if len(row.ProjectsJSON) > 0 {
		if err := json.Unmarshal(row.ProjectsJSON, &rec.Projects); err != nil {
			return nil, fmt.Errorf("history: unmarshal projects: %w", err)
		}
	}
	return rec, nil
}

// List 返回所有摘要,按 StartedAt 倒序。
func (s *Store) List() ([]ScanSummary, error) {
	if s.db == nil {
		return nil, fmt.Errorf("history: nil db")
	}
	rows, err := storage.ListHistorySummaries(s.db)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	out := make([]ScanSummary, 0, len(rows))
	for _, row := range rows {
		ts, tsErr := time.Parse(time.RFC3339Nano, row.StartedAt)
		if tsErr != nil {
			return nil, fmt.Errorf("history: parse started_at %q: %w", row.StartedAt, tsErr)
		}
		scope := row.Scope
		if scope == "global" {
			scope = ""
		}
		out = append(out, ScanSummary{
			ID:            row.ID,
			AgentID:       row.AgentID,
			BatchID:       row.BatchID,
			StartedAt:     ts,
			HealthScore:   row.HealthScore,
			Band:          row.HealthBand,
			FindingCount:  row.FindingCount,
			DetectorAvail: row.DetectorAvail,
			DetectorTotal: row.DetectorTotal,
			Scope:         scope,
			ScopePath:     row.ScopePath,
		})
	}
	return out, nil
}

// Latest 返回最近一条完整记录;无历史返回 (nil, nil)。
func (s *Store) Latest() (*ScanRecord, error) {
	list, err := s.List()
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return s.Get(list[0].ID)
}

// LatestForAgent 返回指定 agent 最近一条完整记录;空 agentID 表示"所有 agent"。
// 优先返回 scope 为 global(含空,即旧记录)的最新记录——dashboard/findings/health
// 需要扫描全貌,而非某次 project/asset 窄范围 rescan(虽可能更晚)。若无 global scope
// 记录,退化为该 agent(或全体)任意 scope 的最新一条,保留"至少展示一些"的语义。
// 无匹配历史返回 (nil, nil)。
func (s *Store) LatestForAgent(agentID string) (*ScanRecord, error) {
	list, err := s.List()
	if err != nil {
		return nil, err
	}
	target := agentID
	// 第一遍:优先取 scope=="" || scope=="global" 的最新一条(agentID 过滤后)
	for _, sum := range list { // List 已按 StartedAt 倒序
		if (target == "" || sum.AgentID == target) && (sum.Scope == "" || sum.Scope == "global") {
			return s.Get(sum.ID)
		}
	}
	// 退化:任意 scope(agentID 过滤后第一条)
	if target == "" {
		if len(list) > 0 {
			return s.Get(list[0].ID)
		}
		return nil, nil
	}
	for _, sum := range list {
		if sum.AgentID == target {
			return s.Get(sum.ID)
		}
	}
	return nil, nil
}

// LatestForAgents 返回每个 agent 最近一次 global scope 完整记录。
// agentIDs 空、或仅含空串(如 []string{""},空 query 的 strings.Split 产物)
// → 返回所有 agent 各自最新 global。
// 与 LatestForAgent 不同:此方法不退化为任意 scope,只取 global(含空 scope)
// 记录;仅有 project/asset scope 的 agent 不出现在结果中。
func (s *Store) LatestForAgents(agentIDs []string) (map[string]*ScanRecord, error) {
	list, err := s.List()
	if err != nil {
		return nil, err
	}
	// 过滤空串:[]string{""}(空 query split 产物)按"所有 agent"处理(与 nil/[] 一致)。
	// 同时容忍 ["a", ""] 这类混合输入,只保留非空 id。
	nonEmpty := make([]string, 0, len(agentIDs))
	for _, id := range agentIDs {
		if id != "" {
			nonEmpty = append(nonEmpty, id)
		}
	}
	agentIDs = nonEmpty
	if len(agentIDs) == 0 {
		// 空 → 从 list 收集所有唯一 agentID
		seen := map[string]bool{}
		for _, sum := range list {
			seen[sum.AgentID] = true
		}
		for id := range seen {
			agentIDs = append(agentIDs, id)
		}
	}
	targets := map[string]bool{}
	for _, id := range agentIDs {
		targets[id] = true
	}
	result := map[string]*ScanRecord{}
	for _, sum := range list { // List 已按 StartedAt 倒序
		if !targets[sum.AgentID] {
			continue
		}
		if _, done := result[sum.AgentID]; done {
			continue // 已取到该 agent 的最新
		}
		if sum.Scope != "" && sum.Scope != "global" {
			continue // 只取 global,不退化为 project/asset
		}
		rec, err := s.Get(sum.ID)
		if err != nil {
			continue
		}
		result[sum.AgentID] = rec
	}
	return result, nil
}

// Delete 删除单条记录。不存在返回 ErrNotFound。
func (s *Store) Delete(id string) error {
	if s.db == nil {
		return fmt.Errorf("history: nil db")
	}
	found, err := storage.DeleteHistory(s.db, id)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	return nil
}
