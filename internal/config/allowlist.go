package config

import (
	"strings"
	"sync"
	"time"

	"code-agent-sentinel/internal/storage"
)

// AllowlistStore 管理运行时拦截放行清单(精确命令字符串)。
// 存 sqlite allowlist_entries 表。
// 持 sync.RWMutex:Load RLock,Save Lock,并发安全。
//
// 安全不变量:只精确整条命令匹配(strings.TrimSpace 后全等),不做通配/正则
// (防危险方向误放行)。panic/异常 → Matches 返回 false(保守,不放行走正常评估)。
//
// 双匹配在调用方编排(见 spec §6.2):本结构 Matches 只做单次精确匹配;
// Task 7 guard_cmd 对原始命令 + normalize 后命令各调一次。
type AllowlistStore struct {
	mu sync.RWMutex
	db *storage.DB
}

// NewAllowlistStore 构造一个指向 db 的放行清单存储。
func NewAllowlistStore(db *storage.DB) *AllowlistStore {
	return &AllowlistStore{db: db}
}

// Load 读取放行清单。db 为 nil 或表空时返回空切片(nil 安全,非 error)。
func (s *AllowlistStore) Load() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := storage.ListAllowlist(s.db)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Command
	}
	return out, nil
}

// Save 全量写回(事务内 DELETE ALL + INSERT)。
func (s *AllowlistStore) Save(list []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	entries := make([]storage.AllowlistEntryRow, len(list))
	for i, cmd := range list {
		entries[i] = storage.AllowlistEntryRow{Command: cmd, CreatedAt: now}
	}
	return storage.ReplaceAllowlist(s.db, entries)
}

// Matches 对命令做精确匹配(strings.TrimSpace 后全等)。
// panic/异常 → false(保守,不放行走正常评估)。不做通配/正则。
// 空命令(trim 后为 "")永不命中:放行空命令无意义且可能误放行。
func (s *AllowlistStore) Matches(command string) (matched bool) {
	defer func() {
		if r := recover(); r != nil {
			matched = false // 异常保守不放行
		}
	}()
	list, err := s.Load()
	if err != nil {
		return false
	}
	target := strings.TrimSpace(command)
	if target == "" {
		return false
	}
	for _, item := range list {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}
