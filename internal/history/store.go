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
			StartedAt   time.Time `json:"started_at"`
			HealthScore *struct {
				Score int    `json:"score"`
				Band  string `json:"band"`
			} `json:"health_score"`
			Findings  []struct{} `json:"findings"`
			Detectors []struct {
				Available bool `json:"available"`
			} `json:"detectors"`
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
			StartedAt:     partial.StartedAt,
			FindingCount:  len(partial.Findings),
			DetectorAvail: avail,
			DetectorTotal: total,
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

// Delete 删除单条记录。不存在返回 ErrNotFound。
func (s *Store) Delete(id string) error {
	err := os.Remove(s.path(id))
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}

// formatID 生成 "<时间戳>-<8hex>" 形式的 ID(内部辅助)。
// 注意:测试中不要调用此函数(它依赖时间),用固定 ID。
func formatID(t time.Time, rand8hex string) string {
	return t.Format("2006-01-02-15-04-05") + "-" + rand8hex
}
