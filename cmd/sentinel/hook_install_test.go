package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallGuardHookNewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	changed, err := InstallGuardHook(path, "/usr/local/bin/sentinel")
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

func TestInstallGuardHookIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := InstallGuardHook(path, "/usr/local/bin/sentinel"); err != nil {
		t.Fatal(err)
	}
	changed, err := InstallGuardHook(path, "/usr/local/bin/sentinel")
	if err != nil || changed {
		t.Fatalf("二次安装应 changed=false(幂等): %v changed=%v", err, changed)
	}
}

func TestInstallGuardHookPreservesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, path, []byte(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"other-hook"}]}]},"foo":"bar"}`))
	changed, err := InstallGuardHook(path, "/usr/local/bin/sentinel")
	if err != nil || !changed {
		t.Fatalf("应 changed=true: %v", err)
	}
	var got map[string]any
	json.Unmarshal(readFile(t, path), &got)
	if got["foo"] != "bar" {
		t.Fatal("应保留既有 foo 字段")
	}
	pre := got["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(pre) != 1 {
		t.Fatalf("应合并到单个 Bash matcher, got %d", len(pre))
	}
	hooks := pre[0].(map[string]any)["hooks"].([]any)
	// sentinel 应置首
	if hooks[0].(map[string]any)["command"] != "/usr/local/bin/sentinel guard" {
		t.Fatalf("sentinel 应置首, got %v", hooks[0])
	}
	if len(hooks) != 2 {
		t.Fatalf("应保留 other-hook, got %d hooks", len(hooks))
	}
}

func TestUninstallGuardHook(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	InstallGuardHook(path, "/usr/local/bin/sentinel")
	changed, err := UninstallGuardHook(path)
	if err != nil || !changed {
		t.Fatalf("卸载应 changed=true: %v", err)
	}
	// 再卸一次:无 sentinel hook → changed=false
	changed, err = UninstallGuardHook(path)
	if err != nil || changed {
		t.Fatalf("二次卸载应 changed=false: %v", err)
	}
}

func readFile(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func writeFile(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
