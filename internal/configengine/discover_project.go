package configengine

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// discoverProjects 发现所有已知项目的项目级资产:settings / .mcp.json / memory /
// skills / commands / agents / scripts(从项目级 hook/command 抽取)。
//
// 遍历 ListProjects() 返回的全部项目(全 agent 发现),缺失目录静默跳过。
// 解析失败被各 parse* 函数内部吞为带 parse_error 的占位资产,不致整体失败。
func (e *Engine) discoverProjects(inv *Inventory) {
	projects, err := e.ListProjects()
	if err != nil || len(projects) == 0 {
		return
	}
	for _, p := range projects {
		if !fileExists(filepath.Join(p.Path, ".claude")) && !fileExists(filepath.Join(p.Path, ".mcp.json")) {
			// 项目目录已不存在(可能 ~/.claude.json 里登记的路径已删),静默跳过。
			continue
		}
		e.discoverOneProject(inv, p)
		inv.Projects = append(inv.Projects, p)
	}
}

// discoverOneProject 发现单个项目的项目级资产(原 discoverProject 主体)。
func (e *Engine) discoverOneProject(inv *Inventory, p Project) {
	d := filepath.Join(p.Path, ".claude")

	if sp := filepath.Join(d, "settings.json"); fileExists(sp) {
		if a, _ := parseSettings(sp, ScopeProject); a != nil {
			inv.Assets = append(inv.Assets, a...)
		}
	}
	if mp := filepath.Join(p.Path, ".mcp.json"); fileExists(mp) {
		if a, _ := parseMCPJSON(mp, ScopeProject); a != nil {
			inv.Assets = append(inv.Assets, a...)
		}
	}
	if a, _ := parseClaudeJSONProjectMCP(e.ClaudeJSON, p.Path, ScopeProject); a != nil {
		inv.Assets = append(inv.Assets, a...)
	}
	if mem, _ := parseMemory(d, ScopeProject); mem != nil {
		inv.Assets = append(inv.Assets, mem...)
	}
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
	// 仅对项目级 hook/command 资产抽取脚本(沿用原偏差注释,防全局脚本重复抽取)。
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
