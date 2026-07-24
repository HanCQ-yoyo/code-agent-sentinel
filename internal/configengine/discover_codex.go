package configengine

import (
	"os"
	"path/filepath"
)

// discoverCodex 发现 Codex CLI 全局资产:config.toml + hooks.json + AGENTS.md + prompts/。
// 项目级发现复用 ~/.claude.json 的 projects 清单(见 discoverCodexProjects)。
//
// 设计:config.toml 走新 parseCodexConfig;hooks.json 复用 parseHooksFromData(对 event
// 名零校验,Codex 的 PascalCase event 直接吃);AGENTS.md 走 codexAgentsMDAsset;
// prompts/ 复用 parseMarkdownDir(目录,每个 .md 一条 skill 资产)。
func (e *Engine) discoverCodex() (Inventory, error) {
	inv := Inventory{}
	codex := e.ClaudeDir // 对 codex agent 即 ~/.codex

	// config.toml:settings + mcp_server + profile
	if p := filepath.Join(codex, "config.toml"); fileExists(p) {
		if a, _ := parseCodexConfig(p, ScopeGlobal); a != nil {
			inv.Assets = append(inv.Assets, a...)
		}
	}
	// hooks.json:Codex 把 hook 命令定义放独立 hooks.json(config.toml 只存 hooks.state
	// 信任哈希,非命令定义)。parseHooksFromData 接受顶层即 hooks map 的布局,Codex
	// 的 PascalCase event(SessionStart/PreToolUse/...)原样保留在 Fields["event"]。
	if p := filepath.Join(codex, "hooks.json"); fileExists(p) {
		if data, err := os.ReadFile(p); err == nil {
			inv.Assets = append(inv.Assets, parseHooksFromData(data, p, ScopeGlobal)...)
		}
	}
	// AGENTS.md:全局指令文件(等同 Claude CLAUDE.md),归 memory 类型。
	if a := codexAgentsMDAsset(filepath.Join(codex, "AGENTS.md"), ScopeGlobal); a != nil {
		inv.Assets = append(inv.Assets, *a)
	}
	// prompts/:可复用提示模板(等同 Claude skills),每个 .md 一条 skill 资产。
	if a, _ := parseMarkdownDir(filepath.Join(codex, "prompts"), AssetSkill, ScopeGlobal); a != nil {
		inv.Assets = append(inv.Assets, a...)
	}

	// 项目级:遍历已知项目读各项目 AGENTS.md。
	e.discoverCodexProjects(&inv)

	inv.Assets = e.filterByEnabledTypes(inv.Assets)
	inv.Duplicates = detectDuplicates(inv.Assets)
	return inv, nil
}

// discoverCodexProjects 遍历已知项目(复用 ~/.claude.json 的 projects 清单),读各项目
// 根目录的 AGENTS.md 作为项目级 memory 资产。
//
// 来源说明:Codex CLI 无项目清单文件,但 sentinel 是多 agent 工具,用户通常同时用
// Claude(~/.claude.json 登记 projects)。此处复用该清单作为"已知项目"来源,只读各项目
// AGENTS.md(不读 .claude)。纯 Codex 用户无 ~/.claude.json → 项目级为空,全局发现照常。
func (e *Engine) discoverCodexProjects(inv *Inventory) {
	projects, err := readProjectList(e.ClaudeJSON)
	if err != nil || len(projects) == 0 {
		return
	}
	for _, p := range projects {
		if !fileExists(p.Path) {
			continue
		}
		if a := codexAgentsMDAsset(filepath.Join(p.Path, "AGENTS.md"), ScopeProject); a != nil {
			inv.Assets = append(inv.Assets, *a)
			inv.Projects = append(inv.Projects, p)
		}
	}
}

// codexAgentsMDAsset 读单个 AGENTS.md 产出 memory 资产。文件不存在返回 nil。
// 不复用 parseMemory(它写死了 CLAUDE.md/CLAUDE.local.md/memory/ 文件名);Codex 用
// AGENTS.md,故单独处理。
func codexAgentsMDAsset(path string, scope Scope) *Asset {
	if !fileExists(path) {
		return nil
	}
	data, _ := os.ReadFile(path)
	a := Asset{Type: AssetMemory, Scope: scope, SourcePath: path, Name: "AGENTS.md", Content: string(data)}
	fillHash(&a)
	return &a
}
