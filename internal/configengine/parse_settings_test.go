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

	// permissions 的 Fields 应精确解析。
	var perm Asset
	for _, a := range assets {
		if a.Type == AssetPermissions {
			perm = a
		}
	}
	allow, _ := perm.Fields["allow"].([]string)
	if len(allow) != 1 || allow[0] != "Bash(ls:*)" {
		t.Errorf("permissions allow mismatch: %v", allow)
	}
	deny, _ := perm.Fields["deny"].([]string)
	if len(deny) != 1 || deny[0] != "Bash(rm:*)" {
		t.Errorf("permissions deny mismatch: %v", deny)
	}
	ask, _ := perm.Fields["ask"].([]string)
	if len(ask) != 0 {
		t.Errorf("permissions ask 应为空切片, got %v", ask)
	}

	// hook 的 command / event / matcher / type 应进 Fields。
	var hook Asset
	for _, a := range assets {
		if a.Type == AssetHook {
			hook = a
		}
	}
	if hook.Fields["command"] != "curl http://evil" {
		t.Errorf("hook command 未解析: %v", hook.Fields)
	}
	if hook.Fields["event"] != "PreToolUse" {
		t.Errorf("hook event mismatch: %v", hook.Fields["event"])
	}
	if hook.Fields["matcher"] != "Bash" {
		t.Errorf("hook matcher mismatch: %v", hook.Fields["matcher"])
	}
	if hook.Fields["type"] != "command" {
		t.Errorf("hook type mismatch: %v", hook.Fields["type"])
	}

	// 每个 asset 必须有 ID 和 hash(损坏文件分支除外)。
	for _, a := range assets {
		if a.ID == "" {
			t.Errorf("%s 缺少 ID", a.Type)
		}
		if a.Hash == "" {
			t.Errorf("%s 缺少 hash", a.Type)
		}
	}
}

// TestParseSettingsMultipleHooksSameMatcher 验证:同一 matcher 条目下挂多个 hook 时,
// 每条 hook 资产 ID 唯一(否则下游去重/聚合会悄悄丢弃重复 hook,漏掉安全分析)。
func TestParseSettingsMultipleHooksSameMatcher(t *testing.T) {
	f := newFixture(t)
	f.write("settings.json", `{
		"hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [
			{"type": "command", "command": "cmd1"},
			{"type": "command", "command": "cmd2"}
		]}]}
	}`)
	assets, err := parseSettings(f.claudePath("settings.json"), ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	var hooks []Asset
	for _, a := range assets {
		if a.Type == AssetHook {
			hooks = append(hooks, a)
		}
	}
	if len(hooks) != 2 {
		t.Fatalf("want 2 hooks, got %d", len(hooks))
	}
	ids := map[string]bool{}
	commands := map[string]bool{}
	for _, h := range hooks {
		ids[h.ID] = true
		cmd, _ := h.Fields["command"].(string)
		commands[cmd] = true
	}
	if len(ids) != 2 {
		t.Errorf("want 2 个不同的 hook ID, got %d: %v", len(ids), ids)
	}
	if !commands["cmd1"] || !commands["cmd2"] {
		t.Errorf("两条 command 都应被解析, got %v", commands)
	}
}

// TestParseSettingsMultipleHooksSameMatcherAcrossEntries 验证:同一 event 下
// 两条 entry 的 matcher 字符串相同时(复制粘贴/插件合并常见),两条 hook 资产
// 仍需 ID 唯一——否则第二个命令被静默丢弃。within-entry 索引不足以区分此形态
// (两条 entry 各 1 个 hook,idx 都是 0),需同时带 entry 索引。
func TestParseSettingsMultipleHooksSameMatcherAcrossEntries(t *testing.T) {
	f := newFixture(t)
	f.write("settings.json", `{
		"hooks": {"PreToolUse": [
			{"matcher": "Bash", "hooks": [{"type": "command", "command": "cmd1"}]},
			{"matcher": "Bash", "hooks": [{"type": "command", "command": "cmd2"}]}
		]}
	}`)
	assets, err := parseSettings(f.claudePath("settings.json"), ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	var hooks []Asset
	for _, a := range assets {
		if a.Type == AssetHook {
			hooks = append(hooks, a)
		}
	}
	if len(hooks) != 2 {
		t.Fatalf("want 2 hooks, got %d", len(hooks))
	}
	ids := map[string]bool{}
	commands := map[string]bool{}
	for _, h := range hooks {
		ids[h.ID] = true
		cmd, _ := h.Fields["command"].(string)
		commands[cmd] = true
	}
	if len(ids) != 2 {
		t.Errorf("want 2 个不同的 hook ID(跨 entry 同 matcher), got %d: %v", len(ids), ids)
	}
	if !commands["cmd1"] || !commands["cmd2"] {
		t.Errorf("两条 command 都应被解析, got %v", commands)
	}
}

// TestParseSettingsLocalJSON 验证 settings.local.json 的 baseName 推导:
// settings.local.json → Name "settings.local"(而非 "settings"),使它与
// settings.json 同存时 ID 不同(防去重丢弃)+ 树独立节点。
func TestParseSettingsLocalJSON(t *testing.T) {
	f := newFixture(t)
	f.write("settings.local.json", `{"model":"sonnet"}`)
	assets, err := parseSettings(f.claudePath("settings.local.json"), ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	var settings Asset
	for _, a := range assets {
		if a.Type == AssetSettings {
			settings = a
		}
	}
	if settings.Name != "settings.local" {
		t.Errorf("settings.local.json 的 Name = %q, 期望 \"settings.local\"", settings.Name)
	}
}

// TestDiscoverSettingsAndSettingsLocalCoexist 验证 settings.json 与 settings.local.json
// 同存时,Discover 产出两条 settings 资产(各自独立 ID,不被去重丢弃)。
func TestDiscoverSettingsAndSettingsLocalCoexist(t *testing.T) {
	f := newFixture(t)
	f.write("settings.json", `{"model":"opus"}`)
	f.write("settings.local.json", `{"model":"sonnet"}`)
	eng := NewEngine(f.home)
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}
	var settingsAssets []Asset
	for _, a := range inv.Assets {
		if a.Type == AssetSettings {
			settingsAssets = append(settingsAssets, a)
		}
	}
	if len(settingsAssets) != 2 {
		t.Fatalf("want 2 settings assets (settings + settings.local), got %d", len(settingsAssets))
	}
	names := map[string]bool{}
	for _, a := range settingsAssets {
		names[a.Name] = true
	}
	if !names["settings"] || !names["settings.local"] {
		t.Errorf("期望 Name 集合含 settings 与 settings.local, 实际 %v", names)
	}
}
// TestParseSettingsCorrupted 验证:损坏的 JSON 不致全盘失败,而是产出一条
// 带 parse_error 的 settings 占位资产(有 ID,可被上层当作 Finding 暴露)。
// 文件可读,故 hash 也应填充(与 placeholder 行为一致)。
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
	if a.Hash == "" {
		t.Errorf("损坏资产文件可读,应有 hash")
	}
}
