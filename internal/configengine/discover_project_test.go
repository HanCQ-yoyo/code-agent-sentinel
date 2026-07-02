package configengine

import (
	"path/filepath"
	"testing"
)

func TestDiscoverProject(t *testing.T) {
	f := newFixture(t)
	f.write("settings.json", `{}`)
	f.writeProject("myproj/.claude/settings.json", `{"model":"sonnet"}`)
	f.writeProject("myproj/.mcp.json", `{"mcpServers":{"p":{"command":"x"}}}`)
	f.writeProject("myproj/CLAUDE.md", `# proj`)

	eng := NewEngine(f.home)
	eng.SelectProject(Project{Path: filepath.Join(f.home, "myproj"), Name: "myproj"})
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[AssetType]int{}
	for _, a := range inv.Assets {
		seen[a.Type]++
	}
	// 全局 + 项目都应有 settings;项目应有 mcp_server
	if seen[AssetSettings] < 2 {
		t.Errorf("settings 应含全局+项目: %d", seen[AssetSettings])
	}
	if seen[AssetMCPServer] < 1 {
		t.Errorf("缺项目 mcp: %d", seen[AssetMCPServer])
	}
	if inv.Project == nil || inv.Project.Name != "myproj" {
		t.Errorf("project 未设置: %+v", inv.Project)
	}
}

// TestDiscoverProjectNoScriptDup 回归测试:全局 hook 引用一个已存在的 .sh 脚本时,
// 选择项目后 Discover() 不应把该脚本重复抽取。
//
// 背景:brief Step 4 原始写法 discoverProject 末尾对完整 inv.Assets 调
// parseScripts,但 Discover() 已对同一列表跑过 parseScripts。parseScripts 的
// seen 去重表是 per-call 的(不跨调用共享),第二次调用会重发全局 hook 引用的
// 已存在脚本 → 同 scope+type+name+path → 同 ID → 重复资产 + detectDuplicates
// 误报。修复:discoverProject 仅对项目 scope 的 hook/command 调 parseScripts。
func TestDiscoverProjectNoScriptDup(t *testing.T) {
	f := newFixture(t)
	// 全局 settings.json 挂一个 PreToolUse hook,引用 home 下的 foo.sh。
	scriptPath := filepath.Join(f.home, "scripts", "foo.sh")
	f.write("settings.json", `{"hooks":{"PreToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"bash `+scriptPath+`"}]}]}}`)
	// 创建被引用的脚本文件(f.writeProject 写在 f.home 下,此处复用其 MkdirAll 语义)。
	f.writeProject("scripts/foo.sh", "#!/bin/bash\necho hi\n")

	// 选择任意项目触发 discoverProject(项目无需有真实资产,只要 e.Project != nil)。
	eng := NewEngine(f.home)
	eng.SelectProject(Project{Path: filepath.Join(f.home, "myproj"), Name: "myproj"})
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}

	// script 资产应恰好 1 个(全局 hook 引用的那个),不被 discoverProject 重复抽取。
	scriptCount := 0
	idCount := map[string]int{}
	for _, a := range inv.Assets {
		if a.Type == AssetScript {
			scriptCount++
			idCount[a.ID]++
		}
	}
	if scriptCount != 1 {
		t.Errorf("期望 1 个 script 资产,实际 %d(全局 hook 引用的脚本被重复抽取)", scriptCount)
	}
	for id, c := range idCount {
		if c > 1 {
			t.Errorf("script ID %s 出现 %d 次,应唯一", id, c)
		}
	}

	// detectDuplicates 不应把 script 报为重复(同 ID 的同一资产不算跨 scope 重复)。
	for _, d := range detectDuplicates(inv.Assets) {
		if d.Type == AssetScript {
			t.Errorf("script 被误报为重复: %+v", d)
		}
	}
}
