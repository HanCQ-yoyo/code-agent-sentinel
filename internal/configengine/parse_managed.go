package configengine

import (
	"encoding/json"
	"os"
)

// parseManagedMCP 解析企业 managed-mcp.json(规格 §2.5/§2.10)。
//
// 产出 scope=managed 的 mcp_server 资产,Fields["managed"]=true——基线规则据此识别
// 企业管理模式(对 managed server 施加不同策略,如「企业管理 server 必须在白名单」)。
// 结构与 .mcp.json / ~/.claude.json 顶层 mcpServers 同构,复用 mcpAssets 标准化。
//
// 文件不存在返回 nil, nil(企业管理文件可选,缺失不算错误);损坏 JSON 返回 1 条带
// parse_error 的占位资产(文件可读故填 hash/mtime,与 parseMCPJSON 损坏分支一致)。
//
// managed-mcp.json 不携带 Content(与 ~/.claude.json 同理:企业管理文件可能含敏感信息,
// UI 改展示结构化字段 name/transport/command/args/url/env)。
func parseManagedMCP(path string, scope Scope) ([]Asset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil // 文件不存在,不算错误
	}
	var doc struct {
		MCPServers map[string]mcpEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		// 损坏文件:产出一条带 parse_error 的占位资产。文件可读故填 hash/mtime;
		// fillHash 内部会设 ID(与 parseMCPJSON/parseClaudeJSONMCP 损坏分支一致)。
		a := Asset{Type: AssetMCPServer, Scope: scope, SourcePath: path, Name: "managed-mcp.json", ParseError: err.Error()}
		fillHash(&a)
		return []Asset{a}, nil
	}
	// fileContent 传空:managed-mcp.json 是企业管理文件,UI 展示结构化字段而非原文
	// (与 parseClaudeJSONMCP 同理)。
	assets := mcpAssets(doc.MCPServers, "", path, scope)
	for i := range assets {
		if assets[i].Fields == nil {
			assets[i].Fields = map[string]any{}
		}
		assets[i].Fields["managed"] = true // 规则用 field: managed, op: eq, value: "true"
	}
	return assets, nil
}
