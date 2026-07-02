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
	return mcpAssets(doc.MCPServers, path, scope), nil
}
