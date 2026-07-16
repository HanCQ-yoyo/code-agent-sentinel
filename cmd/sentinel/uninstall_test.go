package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUninstallCmdRegistered(t *testing.T) {
	cmd := newRootCmd()
	found := false
	for _, c := range cmd.Commands() {
		if c.Use == "uninstall" {
			found = true
		}
	}
	if !found {
		t.Fatal("uninstall 子命令未注册")
	}
}

func TestUninstallRemovesDataDir(t *testing.T) {
	home := t.TempDir()
	dataDir := filepath.Join(home, ".claude-sentinel")
	os.MkdirAll(filepath.Join(dataDir, "history"), 0o755)
	os.WriteFile(filepath.Join(dataDir, "config.yaml"), []byte("home_dir: "+home), 0o600)
	os.WriteFile(filepath.Join(dataDir, "baseline.json"), []byte("{}"), 0o600)
	// ~/.claude 不应被删
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{}"), 0o644)

	err := runUninstall(home, true /*yes*/, false /*keepConfig*/, &strings.Builder{})
	if err != nil {
		t.Fatalf("runUninstall: %v", err)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Errorf("dataDir 应被删除: %v", err)
	}
	if _, err := os.Stat(claudeDir); err != nil {
		t.Errorf("~/.claude 不应被删: %v", err)
	}
}

func TestUninstallKeepConfig(t *testing.T) {
	home := t.TempDir()
	dataDir := filepath.Join(home, ".claude-sentinel")
	os.MkdirAll(filepath.Join(dataDir, "history"), 0o755)
	os.WriteFile(filepath.Join(dataDir, "config.yaml"), []byte("home_dir: "+home), 0o600)
	os.WriteFile(filepath.Join(dataDir, "baseline.json"), []byte("{}"), 0o600)

	err := runUninstall(home, true, true /*keepConfig*/, &strings.Builder{})
	if err != nil {
		t.Fatalf("runUninstall: %v", err)
	}
	// config.yaml 保留
	if _, err := os.Stat(filepath.Join(dataDir, "config.yaml")); err != nil {
		t.Errorf("config.yaml 应保留(keepConfig): %v", err)
	}
	// history / baseline 删除
	if _, err := os.Stat(filepath.Join(dataDir, "history")); !os.IsNotExist(err) {
		t.Errorf("history 应删除: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "baseline.json")); !os.IsNotExist(err) {
		t.Errorf("baseline 应删除: %v", err)
	}
}

func TestUninstallRejectsBadPath(t *testing.T) {
	// home 指向根 → 应拒绝
	err := runUninstall("/", true, false, &strings.Builder{})
	if err == nil {
		t.Error("home=/ 应拒绝(防误删)")
	}
}
