// internal/history/store.go
package history

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrNotFound 表示指定 ID 的记录不存在。
var ErrNotFound = errors.New("history: record not found")

// Store 把扫描记录以 JSON 文件持久化到 dir 目录。
// 纯文件 I/O,不碰 ~/.claude;dir 由调用方注入(生产为 ~/.claude-sentinel/history)。
type Store struct{ dir string }

// NewStore 返回指向 dir 的 Store。dir 不存在时由 Save 创建。
func NewStore(dir string) *Store { return &Store{dir: dir} }

func (s *Store) path(id string) string { return filepath.Join(s.dir, id+".json") }

// Save 原子写入一条记录(临时文件 + rename,防崩溃半写)。
func (s *Store) Save(rec ScanRecord) error {
	if rec.ID == "" {
		return errors.New("history: empty ID")
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, "tmp-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // rename 成功后 tmp 已不存在,Remove 无副作用
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path(rec.ID))
}

// Get 取单条完整记录。不存在返回 ErrNotFound。
func (s *Store) Get(id string) (*ScanRecord, error) {
	data, err := os.ReadFile(s.path(id))
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var rec ScanRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// List 返回所有摘要,按 StartedAt 倒序。不全量加载 JSON 字段,
// 仅解析摘要所需字段(无上限保留下防列表卡顿)。
func (s *Store) List() ([]ScanSummary, error) {
	entries, err := os.ReadDir(s.dir)
	if os.IsNotExist(err) {
		return nil, nil // 空目录:无历史,返回 nil 无错
	}
	if err != nil {
		return nil, err
	}
	var out []ScanSummary
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" || strings.HasPrefix(e.Name(), "tmp-") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		// 只解析摘要字段,丢弃 findings/inventory(用局部结构避免全量反序列化开销)
		var partial struct {
			ID          string    `json:"id"`
			AgentID     string    `json:"agent_id"`
			BatchID     string    `json:"batch_id,omitempty"`
			StartedAt   time.Time `json:"started_at"`
			HealthScore *struct {
				Score int    `json:"score"`
				Band  string `json:"band"`
			} `json:"health_score"`
			Findings  []struct{} `json:"findings"`
			Detectors []struct {
				Available bool `json:"available"`
			} `json:"detectors"`
			Scope string `json:"scope"`
		}
		if err := json.Unmarshal(data, &partial); err != nil {
			continue
		}
		avail, total := 0, len(partial.Detectors)
		for _, d := range partial.Detectors {
			if d.Available {
				avail++
			}
		}
		sum := ScanSummary{
			ID:            partial.ID,
			AgentID:       partial.AgentID,
			BatchID:       partial.BatchID,
			StartedAt:     partial.StartedAt,
			FindingCount:  len(partial.Findings),
			DetectorAvail: avail,
			DetectorTotal: total,
			Scope:         partial.Scope,
		}
		if partial.HealthScore != nil {
			sum.HealthScore = partial.HealthScore.Score
			sum.Band = partial.HealthScore.Band
		}
		out = append(out, sum)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
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
	err := os.Remove(s.path(id))
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}

