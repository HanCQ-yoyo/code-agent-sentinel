package configengine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverGlobalEnumeratesFiles(t *testing.T) {
	f := newFixture(t)
	f.write("settings.json", `{}`)
	f.write("CLAUDE.md", `# hi`)
	f.write("skills/s1/SKILL.md", `---\nname: s1\n---\nbody`)
	f.writeClaudeJSON(`{"mcpServers":{}}`)

	eng := NewEngine(f.home, "")
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[AssetType]bool{}
	for _, a := range inv.Assets {
		seen[a.Type] = true
	}
	// 本任务只占位枚举,但至少要标记 settings/memory/skill 存在
	for _, want := range []AssetType{AssetSettings, AssetMemory, AssetSkill} {
		if !seen[want] {
			t.Errorf("缺少 %s", want)
		}
	}
	for _, a := range inv.Assets {
		if a.Scope != ScopeGlobal {
			t.Errorf("%s scope 不是 global: %s", a.Type, a.Scope)
		}
		if a.Hash == "" {
			t.Errorf("%s 没有 hash", a.Type)
		}
	}
}

func TestDiscoverGlobalCustomClaudeDir(t *testing.T) {
	home := t.TempDir()
	// 自定义 claude 目录(非 home/.claude)
	customClaude := filepath.Join(home, "custom-claude")
	if err := os.MkdirAll(filepath.Join(customClaude, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(customClaude, "skills", "my-skill.md"), []byte("# My Skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// .claude.json 仍在 home(Claude 约定,不随 .claude 移动)
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// home/.claude 故意不放东西:若 claudeDir 未生效会从 home/.claude 发现空
	eng := NewEngine(home, customClaude)
	inv, err := eng.Discover()
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	var found bool
	for _, a := range inv.Assets {
		if a.Type == AssetSkill && a.Name == "my-skill" {
			found = true
		}
	}
	if !found {
		t.Error("应从自定义 claudeDir 发现 skill 资产")
	}
	// .claude.json 路径应仍是 home/.claude.json
	if eng.ClaudeJSON != filepath.Join(home, ".claude.json") {
		t.Errorf("ClaudeJSON 应在 home 不随 claudeDir 移动,got %q", eng.ClaudeJSON)
	}
}

func TestDiscoverGlobalDisabledAssetTypes(t *testing.T) {
	f := newFixture(t)
	f.write("skills/keep.md", "# Keep\n")
	f.write("commands/cmd.md", "# Cmd\n")
	eng := NewEngine(f.home, "") // 默认 home/.claude
	eng.DisabledAssetTypes = []AssetType{AssetSkill}
	inv, err := eng.Discover()
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	for _, a := range inv.Assets {
		if a.Type == AssetSkill {
			t.Errorf("skill 应被禁用过滤,仍发现: %+v", a)
		}
	}
	var hasCmd bool
	for _, a := range inv.Assets {
		if a.Type == AssetCommand {
			hasCmd = true
		}
	}
	if !hasCmd {
		t.Error("command 应保留(未禁用)")
	}
}
