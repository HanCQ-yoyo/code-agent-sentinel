package config

import (
	"code-agent-sentinel/internal/storage"
)

// UserPrefsStore 管理用户偏好(key-value JSON),持久化到 sqlite user_prefs 表。
// 零值安全:db 为 nil 时 Get 返回 ("", nil),Set 静默成功。
type UserPrefsStore struct {
	db *storage.DB
}

func NewUserPrefsStore(db *storage.DB) *UserPrefsStore {
	return &UserPrefsStore{db: db}
}

// Get 读取 key 对应的 JSON 字符串。key 不存在返回 ("", nil)。
func (s *UserPrefsStore) Get(key string) (string, error) {
	if s.db == nil {
		return "", nil
	}
	return storage.GetUserPref(s.db, key)
}

// Set 写入 key 对应的 JSON 字符串。空值也写入(不自动清理)。
func (s *UserPrefsStore) Set(key, value string) error {
	if s.db == nil {
		return nil // 测试场景无 db:静默成功
	}
	return storage.SetUserPref(s.db, key, value)
}
