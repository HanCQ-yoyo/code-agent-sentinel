package configengine

import (
	"os"
	"path/filepath"
	"strings"
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

// TestDiscoverGlobalMCPJSON 验证 L1:全局 ~/.claude/.mcp.json 被发现为 scope=global 的 mcp_server
// (规格 §2.1 全局 MCP 文件,与项目 .mcp.json 同构)。
func TestDiscoverGlobalMCPJSON(t *testing.T) {
	f := newFixture(t)
	os.WriteFile(filepath.Join(f.claude, ".mcp.json"), []byte(`{"mcpServers":{"g":{"command":"x"}}}`), 0o644)
	eng := NewEngine(f.home, "")
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range inv.Assets {
		if a.Type == AssetMCPServer && a.Name == "g" && a.Scope == ScopeGlobal {
			found = true
		}
	}
	if !found {
		t.Fatal("应发现 ~/.claude/.mcp.json 的 MCP server g(scope=global)")
	}
}

// TestDiscoverManagedMCP 验证 L2:~/.claude/managed-mcp.json 被发现为 scope=managed 的 mcp_server,
// 且 Fields["managed"]=true(规格 §2.5/§2.10 企业管理模式标记)。
func TestDiscoverManagedMCP(t *testing.T) {
	f := newFixture(t)
	os.WriteFile(filepath.Join(f.claude, "managed-mcp.json"), []byte(`{"mcpServers":{"m":{"command":"x"}}}`), 0o644)
	eng := NewEngine(f.home, "")
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}
	var managed *Asset
	for i := range inv.Assets {
		a := &inv.Assets[i]
		if a.Type == AssetMCPServer && a.Scope == ScopeManaged {
			managed = a
		}
	}
	if managed == nil {
		t.Fatal("应发现 managed-mcp.json 为 scope=managed 的 mcp_server")
	}
	if managed.Fields["managed"] != true {
		t.Fatalf("managed 标 = %v", managed.Fields["managed"])
	}
}

// TestDiscoverHooksScriptDir 验证 L3:~/.claude/hooks/ 目录下的脚本文件(.sh/.py/.js/.ts/.bash)
// 被枚举为 script 资产(规格 §2.1/§8.1 独立 hook 脚本目录)。子目录不递归。
func TestDiscoverHooksScriptDir(t *testing.T) {
	f := newFixture(t)
	os.MkdirAll(filepath.Join(f.claude, "hooks"), 0o755)
	os.WriteFile(filepath.Join(f.claude, "hooks", "pre.sh"), []byte("#!/bin/sh\necho hi"), 0o644)
	eng := NewEngine(f.home, "")
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range inv.Assets {
		if a.Type == AssetScript && strings.HasSuffix(a.SourcePath, "hooks/pre.sh") {
			found = true
		}
	}
	if !found {
		t.Fatal("应发现 ~/.claude/hooks/pre.sh 为 script 资产")
	}
}

// TestDiscoverHooksScriptNoDuplicateWhenReferencedByHook 验证 Task 7 review Important 修复:
// 当 hook 的 command 字段引用 ~/.claude/hooks/ 下的脚本(绝对路径),且 parseHooksScriptDir
// 已枚举该脚本为 script 资产时,parseScripts 不应再次产出同路径的 script 资产(避免重复入表,
// 否则浪费 secret/dep 检测器成本 + UI 列两条)。
//
// 修复机制:parseScripts 调用时用传入 inv.Assets 中已有 AssetScript 的 SourcePath 预填 seen,
// 跳过 parseHooksScriptDir 已产出的同路径脚本。detectDuplicates 只上报不删除,故若 parseScripts
// 未预填 seen,会留下两条同 type+name 的 script 资产,被 detectDuplicates 误报为重复。
func TestDiscoverHooksScriptNoDuplicateWhenReferencedByHook(t *testing.T) {
	f := newFixture(t)
	hooksDir := filepath.Join(f.claude, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(hooksDir, "pre.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// settings.json 里挂一个 PreToolUse hook,command 以绝对路径引用 hooks/pre.sh。
	// 这样 parseHooksScriptDir 会枚举 hooks/pre.sh,parseScripts 也会从 hook.command 抽到同路径。
	settings := `{"hooks":{"PreToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"bash ` + scriptPath + `"}]}]}}`
	if err := os.WriteFile(filepath.Join(f.claude, "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := NewEngine(f.home, "")
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}

	// 统计 SourcePath == scriptPath 的 script 资产数,应恰为 1。
	var matches []Asset
	for _, a := range inv.Assets {
		if a.Type == AssetScript && filepath.Clean(a.SourcePath) == filepath.Clean(scriptPath) {
			matches = append(matches, a)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("期望 1 个 script 资产(SourcePath=%s),实际 %d: %+v", scriptPath, len(matches), matches)
	}

	// 同路径 script 不应被 detectDuplicates 上报为重复(因只有一条)。
	for _, d := range inv.Duplicates {
		if d.Type != AssetScript {
			continue
		}
		for _, id := range d.AssetIDs {
			for _, m := range matches {
				if m.ID == id {
					t.Errorf("同路径 script 不应被报为 duplicate: %+v", d)
				}
			}
		}
	}
}

// TestDiscoverProjectHooksScriptNoDuplicateWhenReferencedByHook 验证项目级同理:
// 项目 .claude/hooks/ 脚本被项目 settings.json 的 hook 引用时,不重复产出。
// 关键:discover_project.go 把 parseHooksScriptDir 的输出同时并入 projAssets 与 inv.Assets,
// 使 parseScripts 能预填 seen 跳过同路径脚本。
func TestDiscoverProjectHooksScriptNoDuplicateWhenReferencedByHook(t *testing.T) {
	f := newFixture(t)
	proj := filepath.Join(f.home, "myproj")
	hooksDir := filepath.Join(proj, ".claude", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(hooksDir, "deploy.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho deploy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	settings := `{"hooks":{"PostToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"bash ` + scriptPath + `"}]}]}}`
	if err := os.WriteFile(filepath.Join(proj, ".claude", "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	f.writeClaudeJSON(`{"projects":{"` + proj + `":{}}}`)

	eng := NewEngine(f.home, "")
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}

	var matches []Asset
	for _, a := range inv.Assets {
		if a.Type == AssetScript && filepath.Clean(a.SourcePath) == filepath.Clean(scriptPath) {
			matches = append(matches, a)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("期望 1 个项目级 script 资产(SourcePath=%s),实际 %d: %+v", scriptPath, len(matches), matches)
	}
	if matches[0].Scope != ScopeProject {
		t.Errorf("项目级 hooks/ 脚本 scope 应为 project,got %s", matches[0].Scope)
	}
}
