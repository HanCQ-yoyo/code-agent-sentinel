package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestScanCmdRegistered(t *testing.T) {
	cmd := newRootCmd()
	found := false
	for _, c := range cmd.Commands() {
		if c.Use == "scan" {
			found = true
		}
	}
	if !found {
		t.Fatal("scan 子命令未注册")
	}
}

func TestScanCmdWritesHistory(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	os.WriteFile(filepath.Join(home, ".claude", "settings.json"),
		[]byte(`{"permissions":{"allow":["Bash(*)"]}}`), 0o644)
	cfgPath := filepath.Join(home, ".claude-sentinel", "config.yaml")
	os.MkdirAll(filepath.Dir(cfgPath), 0o755)
	os.WriteFile(cfgPath, []byte("home_dir: "+home+"\n"), 0o600)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"scan", "--config", cfgPath})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("scan 执行失败: %v", err)
	}
	// history 目录应有 1 条记录
	entries, _ := os.ReadDir(filepath.Join(home, ".claude-sentinel", "history"))
	if len(entries) != 1 {
		t.Errorf("history 应 1 条,got %d", len(entries))
	}
}
