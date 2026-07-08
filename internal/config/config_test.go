package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadDefaultsWhenMissing(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Bind != "127.0.0.1" {
		t.Errorf("默认 bind: %s", c.Bind)
	}
	if c.Port != 0 {
		t.Errorf("默认 port 应 0(随机): %d", c.Port)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte("bind: 0.0.0.0\nport: 8080\nallowed_cidrs: [\"10.0.0.0/8\"]\nbasic_auth:\n  user: admin\n  password_hash: \"$2a$\"\n"), 0o644)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Bind != "0.0.0.0" || c.Port != 8080 {
		t.Errorf("解析错: %+v", c)
	}
	if len(c.AllowedCIDRs) != 1 || c.AllowedCIDRs[0] != "10.0.0.0/8" {
		t.Errorf("cidrs: %v", c.AllowedCIDRs)
	}
	if c.BasicAuth == nil || c.BasicAuth.User != "admin" {
		t.Errorf("basic auth: %+v", c.BasicAuth)
	}
}

func TestConfigHasNoProjectField(t *testing.T) {
	// --project 启动项已下线,Config 不应再有 Project 字段(若残留则断言失败)。
	var c Config
	_, ok := reflect.TypeOf(c).FieldByName("Project")
	if ok {
		t.Fatal("Config 不应再有 Project 字段(--project 已移除)")
	}
}
