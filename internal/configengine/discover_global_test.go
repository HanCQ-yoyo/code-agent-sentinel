package configengine

import "testing"

func TestDiscoverGlobalEnumeratesFiles(t *testing.T) {
	f := newFixture(t)
	f.write("settings.json", `{}`)
	f.write("CLAUDE.md", `# hi`)
	f.write("skills/s1/SKILL.md", `---\nname: s1\n---\nbody`)
	f.writeClaudeJSON(`{"mcpServers":{}}`)

	eng := NewEngine(f.home)
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
