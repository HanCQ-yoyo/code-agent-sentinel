package configengine

import (
	"os"
	"path/filepath"
	"testing"
)

// codexFixture 在临时目录造一个假 ~/.codex 结构。
func codexFixture(t *testing.T) (home, codex string) {
	t.Helper()
	home = t.TempDir()
	codex = filepath.Join(home, ".codex")
	if err := os.MkdirAll(codex, 0o755); err != nil {
		t.Fatal(err)
	}
	return home, codex
}

func TestDiscoverCodexGlobal(t *testing.T) {
	home, codex := codexFixture(t)
	// config.toml
	os.WriteFile(filepath.Join(codex, "config.toml"), []byte(`model = "gpt-5-codex"
sandbox_mode = "workspace-write"
[mcp_servers.fs]
command = "npx"
`), 0o644)
	// hooks.json(PascalCase event,验证复用 parseHooksFromData)
	os.WriteFile(filepath.Join(codex, "hooks.json"), []byte(`{"PreToolUse":[{"hooks":[{"type":"command","command":"echo hi"}]}]}`), 0o644)
	// AGENTS.md
	os.WriteFile(filepath.Join(codex, "AGENTS.md"), []byte("# Codex 指令\n你是安全助手"), 0o644)
	// prompts/
	os.MkdirAll(filepath.Join(codex, "prompts"), 0o755)
	os.WriteFile(filepath.Join(codex, "prompts", "review.md"), []byte("---\nname: review\n---\n审查代码"), 0o644)

	eng := &Engine{HomeDir: home, ClaudeDir: codex, Kind: "codex"}
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}
	types := map[AssetType]int{}
	for _, a := range inv.Assets {
		types[a.Type]++
	}
	if types[AssetSettings] != 1 {
		t.Fatalf("settings 数 = %d, want 1", types[AssetSettings])
	}
	if types[AssetMCPServer] != 1 {
		t.Fatalf("mcp_server 数 = %d, want 1", types[AssetMCPServer])
	}
	if types[AssetHook] != 1 {
		t.Fatalf("hook 数 = %d, want 1(来自 hooks.json)", types[AssetHook])
	}
	if types[AssetMemory] != 1 {
		t.Fatalf("memory 数 = %d, want 1(AGENTS.md)", types[AssetMemory])
	}
	if types[AssetSkill] != 1 {
		t.Fatalf("skill 数 = %d, want 1(prompts/review.md)", types[AssetSkill])
	}
	// 验证 hook 的 event 保留 PascalCase
	var hook *Asset
	for i := range inv.Assets {
		if inv.Assets[i].Type == AssetHook {
			hook = &inv.Assets[i]
		}
	}
	if hook == nil || hook.Fields["event"] != "PreToolUse" {
		t.Fatalf("hook event 应为 PreToolUse, got %v", hook)
	}
}

func TestDiscoverCodexMissingFilesNoError(t *testing.T) {
	home, codex := codexFixture(t)
	// 只有 config.toml,无 hooks.json/AGENTS.md/prompts
	os.WriteFile(filepath.Join(codex, "config.toml"), []byte(`model = "x"`), 0o644)
	eng := &Engine{HomeDir: home, ClaudeDir: codex, Kind: "codex"}
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Assets) != 1 {
		t.Fatalf("got %d assets, want 1(仅 settings)", len(inv.Assets))
	}
}

func TestDiscoverCodexEmptyDirNoError(t *testing.T) {
	// ~/.codex 存在但空(没 config.toml):0 资产,不报错
	home, codex := codexFixture(t)
	eng := &Engine{HomeDir: home, ClaudeDir: codex, Kind: "codex"}
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Assets) != 0 {
		t.Fatalf("空 ~/.codex 应产 0 资产, got %d", len(inv.Assets))
	}
}

func TestDiscoverCodexDisabledSkillType(t *testing.T) {
	home, codex := codexFixture(t)
	os.WriteFile(filepath.Join(codex, "config.toml"), []byte(`model = "x"`), 0o644)
	os.MkdirAll(filepath.Join(codex, "prompts"), 0o755)
	os.WriteFile(filepath.Join(codex, "prompts", "a.md"), []byte("a"), 0o644)
	eng := &Engine{
		HomeDir: home, ClaudeDir: codex, Kind: "codex",
		DisabledAssetTypes: []AssetType{AssetSkill},
	}
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range inv.Assets {
		if a.Type == AssetSkill {
			t.Fatal("disabled_asset_types=[skill] 时 prompts 不应出现")
		}
	}
}

func TestDiscoverCodexProjectAGENTSMD(t *testing.T) {
	home, codex := codexFixture(t)
	os.WriteFile(filepath.Join(codex, "config.toml"), []byte(`model = "x"`), 0o644)
	// 造一个已知项目(经 ~/.claude.json projects 清单)
	proj := filepath.Join(home, "myproj")
	os.MkdirAll(proj, 0o755)
	os.WriteFile(filepath.Join(proj, "AGENTS.md"), []byte("# 项目指令"), 0o644)
	os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"projects":{"`+proj+`":{}}}`), 0o644)

	eng := &Engine{HomeDir: home, ClaudeDir: codex, ClaudeJSON: filepath.Join(home, ".claude.json"), Kind: "codex"}
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}
	var projMem *Asset
	for i := range inv.Assets {
		if inv.Assets[i].Type == AssetMemory && inv.Assets[i].Scope == ScopeProject {
			projMem = &inv.Assets[i]
		}
	}
	if projMem == nil {
		t.Fatal("应发现项目级 AGENTS.md(scope=project)")
	}
	if projMem.Name != "AGENTS.md" {
		t.Fatalf("项目 memory Name = %q, want AGENTS.md", projMem.Name)
	}
}
