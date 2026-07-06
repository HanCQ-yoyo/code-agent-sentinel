package configengine

import (
	"os"
	"path/filepath"
	"testing"
)

// makePluginFixture 在临时目录造一个版本化插件布局。
// cache/<marketplace>/<plugin>/<version>/.claude-plugin/plugin.json + skills/<name>/SKILL.md
func makePluginFixture(t *testing.T, home, marketplace, plugin, version string, skills []string) {
	t.Helper()
	root := filepath.Join(home, ".claude", "plugins", "cache", marketplace, plugin, version)
	// 清单
	manifest := `{"name":"` + plugin + `","version":"` + version + `","description":"d","author":{"name":"A"}}`
	writeFile(t, filepath.Join(root, ".claude-plugin", "plugin.json"), manifest)
	// skills/<name>/SKILL.md
	for _, s := range skills {
		writeFile(t, filepath.Join(root, "skills", s, "SKILL.md"),
			"---\nname: "+s+"\ndescription: "+s+" skill\n---\n# "+s+"\nbody")
	}
}

// writeFile 是 fixtures_test 没导出的写文件助手;在测试包内直接用 os.WriteFile。
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParsePluginsVersionLayout(t *testing.T) {
	f := newFixture(t)
	makePluginFixture(t, f.home, "claude-plugins-official", "superpowers", "6.1.0", []string{"brainstorming", "tdd"})
	// 多版本:取最高
	makePluginFixture(t, f.home, "claude-plugins-official", "ralph-loop", "1.0.0", []string{"loop"})
	makePluginFixture(t, f.home, "claude-plugins-official", "ralph-loop", "0.9.0", []string{"loop"})

	got, err := parsePlugins(f.claude, ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	// 2 个 plugin 资产
	var plugins []Asset
	var skills []Asset
	for _, a := range got {
		switch a.Type {
		case AssetPlugin:
			plugins = append(plugins, a)
		case AssetSkill:
			skills = append(skills, a)
		}
	}
	if len(plugins) != 2 {
		t.Fatalf("plugins = %d, want 2: %+v", len(plugins), plugins)
	}
	// superpowers 下钻 2 skill,ralph-loop 取最高版 1.0.0 下钻 1 skill
	if len(skills) != 3 {
		t.Fatalf("skills = %d, want 3", len(skills))
	}
	// 验证 plugin 资产 Fields
	var sp Asset
	var rl Asset
	for _, p := range plugins {
		switch p.Name {
		case "superpowers":
			sp = p
		case "ralph-loop":
			rl = p
		}
	}
	if sp.Name == "" {
		t.Fatal("superpowers plugin not found")
	}
	if sp.Fields["version"] != "6.1.0" {
		t.Fatalf("version = %v, want 6.1.0", sp.Fields["version"])
	}
	if sp.Fields["marketplace"] != "claude-plugins-official" {
		t.Fatalf("marketplace = %v, want claude-plugins-official", sp.Fields["marketplace"])
	}
	// 直接断言 ralph-loop 取的是 1.0.0 而非 0.9.0(len(skills)==3 只能证明去重,
	// 不能证明选了高版本)。
	if rl.Name == "" {
		t.Fatal("ralph-loop plugin not found")
	}
	if rl.Fields["version"] != "1.0.0" {
		t.Fatalf("ralph-loop version = %v, want 1.0.0", rl.Fields["version"])
	}
	// skill 是 plugin scope 且标注 plugin 名
	for _, s := range skills {
		if s.Scope != ScopePlugin {
			t.Fatalf("skill %s scope = %v, want plugin", s.Name, s.Scope)
		}
		if s.Fields["plugin"] == nil {
			t.Fatalf("skill %s missing plugin field", s.Name)
		}
	}
}

func TestParsePluginsNoCache(t *testing.T) {
	// 无 cache 目录不致错
	f := newFixture(t)
	got, err := parsePlugins(f.claude, ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
}

// TestParsePluginsSkipsNoManifest 锁定 gopls-lsp 场景:真实插件 gopls-lsp 的
// 版本目录下没有 .claude-plugin/plugin.json,pickHighestVersion 必须跳过。
// 这里造两个版本目录:1.0.0 无清单(只放 README.md),0.9.0 有清单。期望只产
// 1 个 plugin 资产,且取 0.9.0(证明 1.0.0 不是因为版本低被跳过,而是因为无清单)。
func TestParsePluginsSkipsNoManifest(t *testing.T) {
	f := newFixture(t)
	// 1.0.0 版本目录存在但无 .claude-plugin/plugin.json
	noManifestDir := filepath.Join(f.home, ".claude", "plugins", "cache",
		"claude-plugins-official", "gopls-lsp", "1.0.0")
	writeFile(t, filepath.Join(noManifestDir, "README.md"), "no manifest here")
	// 0.9.0 版本目录带清单
	makePluginFixture(t, f.home, "claude-plugins-official", "gopls-lsp", "0.9.0", []string{"gopls"})

	got, err := parsePlugins(f.claude, ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	var plugins []Asset
	for _, a := range got {
		if a.Type == AssetPlugin {
			plugins = append(plugins, a)
		}
	}
	if len(plugins) != 1 {
		t.Fatalf("plugins = %d, want 1 (no-manifest version dir should be skipped): %+v",
			len(plugins), plugins)
	}
	if plugins[0].Name != "gopls-lsp" {
		t.Fatalf("plugin name = %q, want gopls-lsp", plugins[0].Name)
	}
	// 取 0.9.0 而非无清单的 1.0.0 —— 证明跳过是因为无清单,不是因为版本低。
	if plugins[0].Fields["version"] != "0.9.0" {
		t.Fatalf("version = %v, want 0.9.0", plugins[0].Fields["version"])
	}
}
