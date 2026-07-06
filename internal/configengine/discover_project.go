package configengine

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// discoverProject 发现项目级资产:settings / .mcp.json / memory / skills /
// commands / agents / scripts(从项目级 hook/command 抽取)。
//
// 所有解析失败均被各 parse* 函数内部吞为带 parse_error 的占位资产,不致整体失败。
func (e *Engine) discoverProject(inv *Inventory) {
	if e.Project == nil {
		return
	}
	d := filepath.Join(e.Project.Path, ".claude")

	// settings.json:项目级 settings + permissions + hooks。
	if sp := filepath.Join(d, "settings.json"); fileExists(sp) {
		if a, _ := parseSettings(sp, ScopeProject); a != nil {
			inv.Assets = append(inv.Assets, a...)
		}
	}

	// 项目根 .mcp.json:项目级 MCP servers。
	if mp := filepath.Join(e.Project.Path, ".mcp.json"); fileExists(mp) {
		if a, _ := parseMCPJSON(mp, ScopeProject); a != nil {
			inv.Assets = append(inv.Assets, a...)
		}
	}

	// ~/.claude.json 的 projects[path].mcpServers(项目 scope)。
	// 与上面 .mcp.json 互补:.mcp.json 是项目本地提交的,.claude.json 是机器管理文件里
	// 按项目记录的。两者都读,IDs 因 source_path 不同而不同。
	if a, _ := parseClaudeJSONProjectMCP(e.ClaudeJSON, e.Project.Path, ScopeProject); a != nil {
		inv.Assets = append(inv.Assets, a...)
	}

	// memory:项目 .claude/CLAUDE.md + memory/ 目录。
	if mem, _ := parseMemory(d, ScopeProject); mem != nil {
		inv.Assets = append(inv.Assets, mem...)
	}

	// skills / commands / agents markdown 目录。
	for _, sub := range []struct {
		rel string
		typ AssetType
	}{
		{"skills", AssetSkill},
		{"commands", AssetCommand},
		{"agents", AssetAgent},
	} {
		if a, _ := parseMarkdownDir(filepath.Join(d, sub.rel), sub.typ, ScopeProject); a != nil {
			inv.Assets = append(inv.Assets, a...)
		}
	}

	// 仅对项目级 hook/command 资产抽取脚本:Discover 已对全局资产跑过 parseScripts,
	// 若此处再扫全表,parseScripts 的 per-call seen 不跨调用共享,会重发全局 hook
	// 引用的已存在脚本 → 重复 asset ID + detectDuplicates 误报。故只扫项目 scope。
	// (brief Step 4 原写 parseScripts(inv.Assets, d),此为审批通过的偏差,见
	//  TestDiscoverProjectNoScriptDup 回归测试。)
	var projAssets []Asset
	for _, a := range inv.Assets {
		if a.Scope == ScopeProject && (a.Type == AssetHook || a.Type == AssetCommand) {
			projAssets = append(projAssets, a)
		}
	}
	inv.Assets = append(inv.Assets, parseScripts(projAssets, d)...)
}

// readProjectList 从 ~/.claude.json 的 projects 字段列出已知项目。
//
// 文件不存在或损坏时返回 nil, nil(不致错:~/.claude.json 可能尚未创建)。
// 只读 projects 的 key(项目路径),value 暂不解析(P1 不需要项目级配置细节)。
func readProjectList(claudeJSON string) ([]Project, error) {
	data, err := os.ReadFile(claudeJSON)
	if err != nil {
		return nil, nil
	}
	var doc struct {
		Projects map[string]json.RawMessage `json:"projects"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, nil
	}
	var out []Project
	for path := range doc.Projects {
		out = append(out, Project{Path: path, Name: filepath.Base(path)})
	}
	return out, nil
}
