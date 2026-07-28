package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// allowlistFile 是 allowlist.yaml 的磁盘结构。
type allowlistFile struct {
	Allowlist []string `yaml:"allowlist"`
}

// AllowlistStore 管理运行时拦截放行清单(精确命令字符串)。
// 存 ~/.claude-sentinel/allowlist.yaml(独立文件,与 config.yaml 解耦)。
// 原子写(tmp + rename,复用 intercept/store.go 模式,本地复制避免反向依赖)。
// 持 sync.RWMutex:Load RLock,Save Lock,并发安全。
//
// 安全不变量:只精确整条命令匹配(strings.TrimSpace 后全等),不做通配/正则
// (防危险方向误放行)。panic/异常 → Matches 返回 false(保守,不放行走正常评估)。
//
// 双匹配在调用方编排(见 spec §6.2):本结构 Matches 只做单次精确匹配;
// Task 7 guard_cmd 对原始命令 + normalize 后命令各调一次。
type AllowlistStore struct {
	mu   sync.RWMutex
	path string
}

// NewAllowlistStore 构造一个指向 path 的放行清单存储。
func NewAllowlistStore(path string) *AllowlistStore {
	return &AllowlistStore{path: path}
}

// Load 读取放行清单。nil-safe:文件不存在返回空切片(非 nil,无 error),
// 与 config.Load 的"文件不存在=默认值"语义一致。
func (s *AllowlistStore) Load() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var f allowlistFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if f.Allowlist == nil {
		return []string{}, nil
	}
	return f.Allowlist, nil
}

// Save 全量写回(原子:tmp 文件 + rename)。目录不存在则创建(0o700:
// 放行清单属用户安全策略,与 config.Save 同权限)。
func (s *AllowlistStore) Save(list []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(allowlistFile{Allowlist: list})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), "allowlist-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // 兜底:rename 成功后 Remove 是 no-op
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
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
