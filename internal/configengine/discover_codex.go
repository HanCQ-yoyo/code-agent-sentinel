package configengine

import (
	"os"
	"path/filepath"
)

// discoverCodex 发现 Codex CLI 全局资产:config.toml + hooks.json + AGENTS.md + prompts/。
// 项目级发现遍历 Engine.KnownProjects(从 sentinel config 桥接,见 discoverCodexProjects)。
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
	// auth.json:Codex 认证凭据(规格 §3.5)。parseCredentials 扫顶层凭据文件,
	// auth.json 命中 credential 资产(Content 空,不暴露明文)。
	inv.Assets = append(inv.Assets, parseCredentials(codex, ScopeGlobal)...)
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

// discoverCodexProjects 遍历已知项目(Engine.KnownProjects,从 sentinel config 桥接),
// 读各项目根目录的 AGENTS.md + .codex/(C2) 作为项目级资产。
//
// 来源说明:Codex CLI 无项目清单文件(规格 §3)。sentinel 用独立 known_projects 清单,
// 不再借用 Claude 的 ~/.claude.json。纯 Codex 用户项目级发现照常(只要 config 登记了项目)。
// 注意:此处直接读 e.KnownProjects,不经 ListProjects()(后者会回退 ~/.claude.json,
// 与"Codex 不借 Claude 机器文件"目标相悖)。
//
// 防御:跳过项目路径等于家目录的项目(家目录下的 ~/.codex 是全局配置,已在 discoverCodex
// 中作为全局资产发现过,不应再作为项目级资产重复出现)。
func (e *Engine) discoverCodexProjects(inv *Inventory) {
	cleanHome := filepath.Clean(e.HomeDir)
	for _, p := range e.KnownProjects {
		if !fileExists(p.Path) {
			continue
		}
		if filepath.Clean(p.Path) == cleanHome {
			continue // 家目录不是项目,跳过(防 ~/.codex 全局资产被重复发现为项目级)
		}
		if a := codexAgentsMDAsset(filepath.Join(p.Path, "AGENTS.md"), ScopeProject); a != nil {
			inv.Assets = append(inv.Assets, *a)
			inv.Projects = append(inv.Projects, p)
		}
		// C2:项目级 .codex/config.toml 发现(scope=project)。
		e.discoverCodexProject(inv, p)
	}
}

// discoverCodexProject 发现项目级 Codex 配置:<project>/.codex/config.toml(项目 scope)。
// 复用 parseCodexConfig(同结构与字段建模,仅 scope 不同)。文件不存在静默跳过。
// AGENTS.md 由 discoverCodexProjects 直接读;此处只读 .codex/config.toml(规格 §3.1 可选目录)。
func (e *Engine) discoverCodexProject(inv *Inventory, p Project) {
	pc := filepath.Join(p.Path, ".codex", "config.toml")
	if !fileExists(pc) {
		return
	}
	if a, _ := parseCodexConfig(pc, ScopeProject); a != nil {
		inv.Assets = append(inv.Assets, a...)
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
