// internal/intercept/store.go
package intercept

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrNotFound 表示指定 ID 的拦截记录不存在。
var ErrNotFound = errors.New("intercept: record not found")

// Store 把拦截记录以 JSON 文件持久化到 dir(镜像 history.Store)。
// 纯文件 I/O,不碰 ~/.claude;dir 由调用方注入(生产 ~/.claude-sentinel/intercept)。
type Store struct{ dir string }

func NewStore(dir string) *Store { return &Store{dir: dir} }

func (s *Store) path(id string) string { return filepath.Join(s.dir, id+".json") }

// Append 原子写一条记录(临时文件 + rename,防崩溃半写)。
func (s *Store) Append(rec InterceptRecord) error {
	if rec.ID == "" {
		return errors.New("intercept: empty ID")
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
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path(rec.ID))
}

func (s *Store) Get(id string) (*InterceptRecord, error) {
	data, err := os.ReadFile(s.path(id))
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var rec InterceptRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// List 返回所有记录,按 Timestamp 倒序(最新在前)。
func (s *Store) List() ([]InterceptRecord, error) {
	entries, err := os.ReadDir(s.dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []InterceptRecord
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" || strings.HasPrefix(e.Name(), "tmp-") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var rec InterceptRecord
		if err := json.Unmarshal(data, &rec); err == nil {
			out = append(out, rec)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.After(out[j].Timestamp) })
	return out, nil
}

func (s *Store) Delete(id string) error {
	err := os.Remove(s.path(id))
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}
