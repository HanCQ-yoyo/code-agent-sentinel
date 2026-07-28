package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallCodexHookNewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	changed, err := InstallCodexHook(path, "/usr/local/bin/sentinel")
	if err != nil || !changed {
		t.Fatalf("新建应 changed=true: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(readFile(t, path), &got); err != nil {
		t.Fatal(err)
	}
	pre := got["hooks"].(map[string]any)["PreToolUse"].([]any)[0].(map[string]any)
	if pre["matcher"] != "Bash" {
		t.Fatalf("matcher 应 Bash, got %v", pre["matcher"])
	}
	cmd := pre["hooks"].([]any)[0].(map[string]any)["command"]
	if cmd != "/usr/local/bin/sentinel guard" {
		t.Fatalf("command 不对: %v", cmd)
	}
}

func TestInstallCodexHookIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	if _, err := InstallCodexHook(path, "/usr/local/bin/sentinel"); err != nil {
		t.Fatal(err)
	}
	changed, err := InstallCodexHook(path, "/usr/local/bin/sentinel")
	if err != nil || changed {
		t.Fatalf("二次安装应 changed=false(幂等): %v changed=%v", err, changed)
	}
}

func TestUninstallCodexHook(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	InstallCodexHook(path, "/usr/local/bin/sentinel")
	changed, err := UninstallCodexHook(path)
	if err != nil || !changed {
		t.Fatalf("卸载应 changed=true: %v", err)
	}
	changed, err = UninstallCodexHook(path)
	if err != nil || changed {
		t.Fatalf("二次卸载应 changed=false: %v", err)
	}
}

// readFile/writeFile 复用 hook_install_test.go 的 helper(同包 main)。
var _ = os.ReadFile
