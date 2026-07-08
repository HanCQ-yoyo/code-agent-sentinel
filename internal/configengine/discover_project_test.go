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
	// 在 ~/.claude.json 登记 myproj 为已知项目(ListProjects 读 projects 的 key)
	f.writeClaudeJSON(`{"projects":{"` + filepath.Join(f.home, "myproj") + `":{}}}`)

	eng := NewEngine(f.home)
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
	if len(inv.Projects) != 1 || inv.Projects[0].Name != "myproj" {
		t.Errorf("Projects 应含 myproj: %+v", inv.Projects)
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
	// 给 myproj 一个 .claude/settings.json,确保 discoverOneProject 实际运行
	// (discoverProjects 的守卫跳过既无 .claude 又无 .mcp.json 的项目)。
	f.writeProject("myproj/.claude/settings.json", `{}`)
	// 在 ~/.claude.json 登记 myproj 为已知项目(ListProjects 读 projects 的 key)。
	f.writeClaudeJSON(`{"projects":{"` + filepath.Join(f.home, "myproj") + `":{}}}`)

	eng := NewEngine(f.home)
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}

	// script 资产应恰好 1 个(全局 hook 引用的那个),不被 discoverOneProject 重复抽取。
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

// TestDiscoverProjectsNoCrossProjectScriptDup 回归测试:多项目循环下,一个项目的
// hook 以绝对路径引用 home 下脚本时,处理后续项目不应把该脚本重复抽取。
//
// 背景:discoverProjects 遍历所有已知项目调用 discoverOneProject,后者末尾的
// projAssets 过滤原扫全部 inv.Assets 的项目级 hook。第 2 个项目处理时,第 1 个
// 项目的 hook 仍在 inv.Assets 里,parseScripts 的 seen 去重 per-call(不跨调用),
// 绝对路径脚本被重发 → 同 ID → detectDuplicates 误报 script 重复。
// 修复:discoverOneProject 用 start:=len(inv.Assets) 捕获本次新增资产,只扫
// inv.Assets[start:],避免重扫前面项目的 hook。
func TestDiscoverProjectsNoCrossProjectScriptDup(t *testing.T) {
	f := newFixture(t)
	// 共享脚本放在 home 下(项目外),myproj 的 hook 以绝对路径引用。
	scriptPath := filepath.Join(f.home, "scripts", "shared.sh")
	f.writeProject("scripts/shared.sh", "#!/bin/bash\necho shared\n")
	// myproj 的 settings 挂一个 PreToolUse hook,引用绝对路径脚本。
	f.writeProject("myproj/.claude/settings.json",
		`{"hooks":{"PreToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"bash `+scriptPath+`"}]}]}}`)
	// otherproj 仅有空 settings,确保 discoverProjects 不跳过它(存在 .claude)。
	f.writeProject("otherproj/.claude/settings.json", `{}`)
	// 在 ~/.claude.json 登记两个项目(readProjectList 读 projects 的 key,绝对路径)。
	f.writeClaudeJSON(`{"projects":{"` + filepath.Join(f.home, "myproj") + `":{},"` + filepath.Join(f.home, "otherproj") + `":{}}}`)

	eng := NewEngine(f.home)
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}

	// script 资产应恰好 1 个(myproj hook 引用的共享脚本),不被 otherproj 处理时重复抽取。
	scriptCount := 0
	idCount := map[string]int{}
	for _, a := range inv.Assets {
		if a.Type == AssetScript {
			scriptCount++
			idCount[a.ID]++
		}
	}
	if scriptCount != 1 {
		t.Errorf("期望 1 个 script 资产,实际 %d(多项目循环下绝对路径脚本被重复抽取)", scriptCount)
	}
	for id, c := range idCount {
		if c > 1 {
			t.Errorf("script ID %s 出现 %d 次,应唯一", id, c)
		}
	}

	// detectDuplicates 不应把 script 报为重复。
	for _, d := range detectDuplicates(inv.Assets) {
		if d.Type == AssetScript {
			t.Errorf("script 被误报为重复: %+v", d)
		}
	}
}
