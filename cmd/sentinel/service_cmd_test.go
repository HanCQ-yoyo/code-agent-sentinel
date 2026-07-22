package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestServiceInstallDryRun:--dry-run 不实际 systemctl enable,只生成单元文件 + 写 token。
func TestServiceInstallDryRun(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte("language: en\n"), 0o644)
	// 模拟二进制路径(无需真实存在,生成器只插值)
	exePath := filepath.Join(dir, "sentinel")
	opts := serviceInstallOpts{
		Home:     dir,
		CfgPath:  cfgPath,
		UserMode: true,
		DryRun:   true,
		ExePath:  exePath,
	}
	tok, err := runServiceInstall(opts)
	if err != nil {
		t.Fatalf("install 失败: %v", err)
	}
	if tok == "" {
		t.Error("应返回/生成 token")
	}
	// 单元文件应已写入(linux 上)
	if runtime.GOOS == "linux" {
		unitPath := filepath.Join(dir, ".config", "systemd", "user", "sentinel.service")
		b, err := os.ReadFile(unitPath)
		if err != nil {
			t.Fatalf("单元文件未生成: %v", err)
		}
		if !strings.Contains(string(b), "ExecStart=") {
			t.Error("单元文件缺 ExecStart")
		}
	}
	// config token 应已写入
	cfg, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(cfg), "token:") {
		t.Error("config 应含 token 字段")
	}
}

// TestScBinPathArg 验证 sc.exe 的 binPath= 值引用逻辑:含空格的路径加双引号,
// 否则原样返回。Windows 上 Program Files 等路径不加引号会致 sc.exe "syntax incorrect"。
func TestScBinPathArg(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`C:\sentinel.exe`, `C:\sentinel.exe`},
		{`C:\Program Files\sentinel.exe`, `"C:\Program Files\sentinel.exe"`},
		{`/usr/local/bin/sentinel`, `/usr/local/bin/sentinel`},
		{`C:\Users\John Doe\sentinel.exe`, `"C:\Users\John Doe\sentinel.exe"`},
		{`path with	tab`, `"path with	tab"`},
	}
	for _, c := range cases {
		got := scBinPathArg(c.in)
		if got != c.want {
			t.Errorf("scBinPathArg(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
