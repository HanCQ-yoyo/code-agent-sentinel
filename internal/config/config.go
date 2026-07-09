package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type BasicAuth struct {
	User         string `yaml:"user"`
	PasswordHash string `yaml:"password_hash"` // bcrypt
}

type Config struct {
	Bind         string     `yaml:"bind"`
	Port         int        `yaml:"port"`
	AllowedCIDRs []string   `yaml:"allowed_cidrs"`
	BasicAuth    *BasicAuth `yaml:"basic_auth"`
	HomeDir      string     `yaml:"home_dir"` // 覆盖 ~/.claude 的 home
	// DirTags 用户对「目录标签」的显式覆盖:key=相对 .claude 根的路径,value=标签。
	// 默认标签见 DefaultDirTags();生效标签由 ResolveDirTag 合并。
	// 空表示用户未自定义,全用默认。见 dir_tags.go。
	DirTags DirTags `yaml:"dir_tags"`
}

func DefaultConfig() *Config {
	return &Config{Bind: "127.0.0.1", Port: 0}
}

// DefaultPath 返回 ~/.claude-sentinel/config.yaml。
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude-sentinel", "config.yaml"), nil
}

// Load 从 path 加载配置;文件不存在返回默认。
func Load(path string) (*Config, error) {
	c := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Save 将配置写回 path(覆盖写)。目录不存在则创建(0o700:含可能的凭据)。
// 用于 /api/dir-tags 等运行期回写用户偏好。
func Save(path string, c *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
