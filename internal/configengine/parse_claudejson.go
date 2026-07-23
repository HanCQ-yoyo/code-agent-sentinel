package configengine

import (
	"encoding/json"
	"os"
)

// parseClaudeJSONMCP 解析 ~/.claude.json 顶层 mcpServers(机器管理文件,只读)。
//
// 文件不存在时返回 nil, nil(文件可能不存在,不算错误),Discover() 可无条件调用。
// 文件存在但损坏:返回一条带 parse_error 的占位资产,避免真实损坏文件被静默吞掉。
func parseClaudeJSONMCP(path string, scope Scope) ([]Asset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil // 文件可能不存在,不算错误
	}
	var doc struct {
		MCPServers map[string]mcpEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		// 损坏文件:产出一条带 parse_error 的占位资产。文件可读故填 hash/mtime;
		// fillHash 内部会设 ID(与 parseMCPJSON 损坏分支一致)。
		a := Asset{Type: AssetMCPServer, Scope: scope, SourcePath: path, Name: ".claude.json", ParseError: err.Error()}
		fillHash(&a)
		return []Asset{a}, nil
	}
	// Content 留空:.claude.json 是机器管理大文件(含 projects/history 等),
	// 不宜作 MCP server 资产的全文本展示;UI 改展示结构化字段。见 mcpAssets 注释。
	return mcpAssets(doc.MCPServers, "", path, scope), nil
}

// parseClaudeJSONProjectMCP 解析 ~/.claude.json 的 projects[projectPath].mcpServers
// (project scope)。文件不存在/无该 project/无 mcpServers 返回 nil(不致错)。
// 损坏文件返回一条带 parse_error 的占位资产。
func parseClaudeJSONProjectMCP(path string, projectPath string, scope Scope) ([]Asset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil // 文件可能不存在,不算错误
	}
	var doc struct {
		Projects map[string]struct {
			MCPServers map[string]mcpEntry `json:"mcpServers"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		a := Asset{Type: AssetMCPServer, Scope: scope, SourcePath: path, Name: ".claude.json", ParseError: err.Error()}
		fillHash(&a)
		return []Asset{a}, nil
	}
	proj, ok := doc.Projects[projectPath]
	if !ok {
		return nil, nil
	}
	return mcpAssets(proj.MCPServers, "", path, scope), nil
}
