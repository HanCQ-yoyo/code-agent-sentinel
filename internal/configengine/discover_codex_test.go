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
	// 造一个已知项目(经 Engine.KnownProjects 注入,不经 ~/.claude.json)
	proj := filepath.Join(home, "myproj")
	os.MkdirAll(proj, 0o755)
	os.WriteFile(filepath.Join(proj, "AGENTS.md"), []byte("# 项目指令"), 0o644)

	eng := &Engine{
		HomeDir: home, ClaudeDir: codex, Kind: "codex",
		KnownProjects: []Project{{Path: proj, Name: "myproj"}},
	}
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
	// 关键:确认无 ~/.claude.json 也正常工作(纯 Codex 用户)
	if _, err := os.Stat(filepath.Join(home, ".claude.json")); err == nil {
		t.Fatal("测试不应创建 ~/.claude.json(纯 Codex 用户场景)")
	}
}

// TestCodexDiscoverIgnoresClaudeJSON:即使存在 ~/.claude.json 且含 projects,
// Codex(KnownProjects 空)也不应借它发现项目级资产。
func TestCodexDiscoverIgnoresClaudeJSON(t *testing.T) {
	home, codex := codexFixture(t)
	os.WriteFile(filepath.Join(codex, "config.toml"), []byte(`model = "x"`), 0o644)
	// 即使存在 ~/.claude.json 且含 projects,Codex 也不应借它发现项目
	claudeProj := filepath.Join(home, "claudeonly")
	os.MkdirAll(claudeProj, 0o755)
	os.WriteFile(filepath.Join(claudeProj, "AGENTS.md"), []byte("# claude 项目"), 0o644)
	os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"projects":{"`+claudeProj+`":{}}}`), 0o644)

	eng := &Engine{HomeDir: home, ClaudeDir: codex, Kind: "codex"} // KnownProjects 空
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range inv.Assets {
		if a.Type == AssetMemory && a.Scope == ScopeProject {
			t.Fatalf("Codex 不应借 ~/.claude.json 发现项目级资产, got %+v", a)
		}
	}
}

// TestListProjectsFallsBackToClaudeJSON:KnownProjects 空 → 回退 ~/.claude.json projects。
// 防止 Task 4 的 ListProjects 回退策略退化(Claude 零破坏)。
func TestListProjectsFallsBackToClaudeJSON(t *testing.T) {
	home := t.TempDir()
	proj := filepath.Join(home, "myproj")
	os.MkdirAll(proj, 0o755)
	os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"projects":{"`+proj+`":{}}}`), 0o644)
	eng := &Engine{HomeDir: home, ClaudeDir: filepath.Join(home, ".claude"), ClaudeJSON: filepath.Join(home, ".claude.json")}
	// KnownProjects 空 → 回退 ~/.claude.json
	got, err := eng.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != proj {
		t.Fatalf("回退应读到 ~/.claude.json projects, got %+v", got)
	}
}

// TestListProjectsPrefersKnownProjects:KnownProjects 非空 → 优先返回,忽略 ~/.claude.json。
func TestListProjectsPrefersKnownProjects(t *testing.T) {
	home := t.TempDir()
	os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"projects":{"/should-not-appear":{}}}`), 0o644)
	eng := &Engine{
		HomeDir: home, ClaudeDir: filepath.Join(home, ".claude"), ClaudeJSON: filepath.Join(home, ".claude.json"),
		KnownProjects: []Project{{Path: "/known", Name: "known"}},
	}
	got, _ := eng.ListProjects()
	if len(got) != 1 || got[0].Path != "/known" {
		t.Fatalf("KnownProjects 非空应优先, got %+v", got)
	}
}

// TestCodexProjectDotCodexConfig 验证 C2:项目级 <project>/.codex/config.toml 发现。
// KnownProjects 注入的项目下放 .codex/config.toml(含 sandbox_mode),应产出 scope=project
// 的 settings 资产且 Fields["sandbox_mode"] 正确。
func TestCodexProjectDotCodexConfig(t *testing.T) {
	home, codex := codexFixture(t)
	os.WriteFile(filepath.Join(codex, "config.toml"), []byte(`model = "x"`), 0o644)
	proj := filepath.Join(home, "myproj")
	dotCodex := filepath.Join(proj, ".codex")
	if err := os.MkdirAll(dotCodex, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dotCodex, "config.toml"), []byte(`sandbox_mode = "read-only"`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := &Engine{
		HomeDir: home, ClaudeDir: codex, Kind: "codex",
		KnownProjects: []Project{{Path: proj, Name: "myproj"}},
	}
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}
	// 项目级 .codex/config.toml 应产出 scope=project 的 settings 资产
	var projSettings *Asset
	for i := range inv.Assets {
		a := &inv.Assets[i]
		if a.Type == AssetSettings && a.Scope == ScopeProject {
			projSettings = a
		}
	}
	if projSettings == nil {
		t.Fatal("应发现项目级 .codex/config.toml(scope=project settings)")
	}
	if projSettings.Fields["sandbox_mode"] != "read-only" {
		t.Fatalf("项目 settings sandbox_mode = %v, want read-only", projSettings.Fields["sandbox_mode"])
	}
}
