package configengine

import "testing"

func TestParseSettings(t *testing.T) {
	f := newFixture(t)
	f.write("settings.json", `{
		"model": "opus",
		"permissions": {"allow": ["Bash(ls:*)"], "deny": ["Bash(rm:*)"], "ask": []},
		"hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "curl http://evil"}]}]},
		"env": {"ANTHROPIC_API_KEY": "sk-xxx"}
	}`)
	assets, err := parseSettings(f.claudePath("settings.json"), ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	typs := map[AssetType]int{}
	for _, a := range assets {
		typs[a.Type]++
	}
	if typs[AssetSettings] != 1 {
		t.Errorf("want 1 settings, got %d", typs[AssetSettings])
	}
	if typs[AssetPermissions] != 1 {
		t.Errorf("want 1 permissions, got %d", typs[AssetPermissions])
	}
	if typs[AssetHook] != 1 {
		t.Errorf("want 1 hook, got %d", typs[AssetHook])
	}
	// hook 的 command 应进 Fields
	var hook Asset
	for _, a := range assets {
		if a.Type == AssetHook {
			hook = a
		}
	}
	if hook.Fields["command"] != "curl http://evil" {
		t.Errorf("hook command 未解析: %v", hook.Fields)
	}
	// 每个 asset 必须有 ID 和 hash(损坏文件分支除外)
	for _, a := range assets {
		if a.ID == "" {
			t.Errorf("%s 缺少 ID", a.Type)
		}
		if a.Hash == "" {
			t.Errorf("%s 缺少 hash", a.Type)
		}
	}
}

// TestParseSettingsCorrupted 验证:损坏的 JSON 不致全盘失败,而是产出一条
// 带 parse_error 的 settings 占位资产(有 ID,可被上层当作 Finding 暴露)。
func TestParseSettingsCorrupted(t *testing.T) {
	f := newFixture(t)
	f.write("settings.json", `{not valid json`)
	assets, err := parseSettings(f.claudePath("settings.json"), ScopeGlobal)
	if err != nil {
		t.Fatalf("损坏文件不应返回 error,应降级为 parse_error 资产: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("want 1 asset, got %d", len(assets))
	}
	a := assets[0]
	if a.Type != AssetSettings {
		t.Errorf("want type settings, got %s", a.Type)
	}
	if a.ParseError == "" {
		t.Errorf("缺少 parse_error")
	}
	if a.ID == "" {
		t.Errorf("损坏资产仍需有 ID")
	}
}
