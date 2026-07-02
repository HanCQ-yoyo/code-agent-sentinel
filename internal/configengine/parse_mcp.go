package configengine

import (
	"encoding/json"
	"os"
)

// mcpEntry 是单个 MCP server 在 .mcp.json / ~/.claude.json 里的原始形态。
type mcpEntry struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	URL     string            `json:"url"`
	Env     map[string]string `json:"env"`
}

// parseMCPJSON 解析项目 .mcp.json 的 mcpServers。
//
// 损坏文件不致失败:返回一条带 parse_error 的 mcp_server 占位资产(有 ID/hash,
// 可被上层当作 Finding 暴露)。文件不存在则返回 error,由调用方决定是否忽略。
func parseMCPJSON(path string, scope Scope) ([]Asset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		MCPServers map[string]mcpEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		// 损坏文件:产出一条带 parse_error 的占位资产。文件可读故填 hash/mtime;
		// fillHash 内部会设 ID(与 parseSettings 损坏分支一致)。
		a := Asset{Type: AssetMCPServer, Scope: scope, SourcePath: path, Name: ".mcp.json", ParseError: err.Error()}
		fillHash(&a)
		return []Asset{a}, nil
	}
	return mcpAssets(doc.MCPServers, path, scope), nil
}

// mcpAssets 把 mcpServers 映射成 []Asset,每条 server 一条资产。
// transport 缺省时由 command/url 推断:有 command → stdio,有 url → http;
// 显式 type 优先。name/path/scope 共同决定 ID,故同名 server 在不同 scope/path
// 下 ID 不同,不会互相覆盖。
func mcpAssets(m map[string]mcpEntry, path string, scope Scope) []Asset {
	var out []Asset
	for name, e := range m {
		transport := e.Type
		if transport == "" {
			if e.Command != "" {
				transport = "stdio"
			} else if e.URL != "" {
				transport = "http"
			}
		}
		a := Asset{Type: AssetMCPServer, Scope: scope, SourcePath: path, Name: name}
		a.Fields = map[string]any{
			"name":      name,
			"transport": transport,
			"command":   e.Command,
			"args":      e.Args,
			"url":       e.URL,
			"env":       e.Env,
		}
		fillHash(&a)
		out = append(out, a)
	}
	return out
}
