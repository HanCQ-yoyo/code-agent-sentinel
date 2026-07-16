package configengine

import (
	"path/filepath"
	"testing"
)

// TestParseMCPJSON 验证项目 .mcp.json 的基础解析:command/args/env 进 Fields,
// transport 在缺省时由 command 推断为 stdio。
func TestParseMCPJSON(t *testing.T) {
	f := newFixture(t)
	f.writeProject("proj/.mcp.json", `{"mcpServers":{"evil":{"command":"npx","args":["x"],"env":{"TOKEN":"t"}}}}`)
	assets, err := parseMCPJSON(filepath.Join(f.home, "proj", ".mcp.json"), ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].Name != "evil" {
		t.Fatalf("want 1 mcp 'evil', got %+v", assets)
	}
	if assets[0].Fields["command"] != "npx" {
		t.Errorf("command 未解析: %v", assets[0].Fields)
	}
	// 缺省 type + 有 command → transport 推断为 stdio。
	if assets[0].Fields["transport"] != "stdio" {
		t.Errorf("transport 应推断为 stdio, got %v", assets[0].Fields["transport"])
	}
	args, _ := assets[0].Fields["args"].([]string)
	if len(args) != 1 || args[0] != "x" {
		t.Errorf("args 未解析: %v", assets[0].Fields["args"])
	}
	env, _ := assets[0].Fields["env"].(map[string]string)
	if env["TOKEN"] != "t" {
		t.Errorf("env 未解析: %v", assets[0].Fields["env"])
	}
	if assets[0].Scope != ScopeProject {
		t.Errorf("scope 应为 project, got %s", assets[0].Scope)
	}
	if assets[0].ID == "" || assets[0].Hash == "" {
		t.Errorf("缺少 ID/hash: %+v", assets[0])
	}
}

// TestParseMCPJSONHTTPTransport 验证:url 存在且无 type 时,transport 推断为 http。
func TestParseMCPJSONHTTPTransport(t *testing.T) {
	f := newFixture(t)
	f.writeProject(".mcp.json", `{"mcpServers":{"gmail":{"type":"http","url":"https://x/mcp"}}}`)
	assets, err := parseMCPJSON(filepath.Join(f.home, ".mcp.json"), ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 {
		t.Fatalf("want 1 asset, got %d", len(assets))
	}
	if assets[0].Fields["transport"] != "http" {
		t.Errorf("transport 应为 http, got %v", assets[0].Fields["transport"])
	}
	if assets[0].Fields["url"] != "https://x/mcp" {
		t.Errorf("url 未解析: %v", assets[0].Fields["url"])
	}
}

// TestParseMCPJSONExplicitType 验证:显式 type 优先于 command/url 推断。
func TestParseMCPJSONExplicitType(t *testing.T) {
	f := newFixture(t)
	f.writeProject(".mcp.json", `{"mcpServers":{"s":{"type":"sse","command":"npx","url":"https://x"}}}`)
	assets, err := parseMCPJSON(filepath.Join(f.home, ".mcp.json"), ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 {
		t.Fatalf("want 1 asset, got %d", len(assets))
	}
	if assets[0].Fields["transport"] != "sse" {
		t.Errorf("显式 type 应优先, got %v", assets[0].Fields["transport"])
	}
}

// TestParseMCPJSONCorrupted 验证:损坏的 .mcp.json 不致失败,降级为带 parse_error
// 的占位资产(有 ID/hash,可被上层当作 Finding 暴露)。文件可读故 hash 应填充。
func TestParseMCPJSONCorrupted(t *testing.T) {
	f := newFixture(t)
	f.writeProject(".mcp.json", `{not valid json`)
	assets, err := parseMCPJSON(filepath.Join(f.home, ".mcp.json"), ScopeProject)
	if err != nil {
		t.Fatalf("损坏文件不应返回 error,应降级为 parse_error 资产: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("want 1 asset, got %d", len(assets))
	}
	a := assets[0]
	if a.Type != AssetMCPServer {
		t.Errorf("want type mcp_server, got %s", a.Type)
	}
	if a.ParseError == "" {
		t.Errorf("缺少 parse_error")
	}
	if a.ID == "" {
		t.Errorf("损坏资产仍需有 ID")
	}
	if a.Hash == "" {
		t.Errorf("损坏资产文件可读,应有 hash")
	}
}

// TestParseMCPJSONMissingFile 验证:文件不存在时返回 error(由调用方决定是否忽略)。
func TestParseMCPJSONMissingFile(t *testing.T) {
	assets, err := parseMCPJSON("/nonexistent/.mcp.json", ScopeProject)
	if err == nil {
		t.Errorf("文件不存在应返回 error")
	}
	if assets != nil {
		t.Errorf("error 时 assets 应为 nil, got %+v", assets)
	}
}

// TestParseClaudeJSONMCP 验证 ~/.claude.json 顶层 mcpServers 解析。
func TestParseClaudeJSONMCP(t *testing.T) {
	f := newFixture(t)
	f.writeClaudeJSON(`{"mcpServers":{"gmail":{"type":"http","url":"https://x/mcp"}}}`)
	assets, err := parseClaudeJSONMCP(f.cj, ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].Name != "gmail" {
		t.Fatalf("got %+v", assets)
	}
	if assets[0].Fields["transport"] != "http" {
		t.Errorf("transport: %v", assets[0].Fields)
	}
	if assets[0].Scope != ScopeGlobal {
		t.Errorf("scope 应为 global, got %s", assets[0].Scope)
	}
}

// TestParseClaudeJSONMCPMissingFile 验证:~/.claude.json 不存在时返回 nil, nil
// (文件可能不存在,不算错误),Discover() 可无条件调用。
func TestParseClaudeJSONMCPMissingFile(t *testing.T) {
	assets, err := parseClaudeJSONMCP("/nonexistent/.claude.json", ScopeGlobal)
	if err != nil {
		t.Errorf("文件不存在不应返回 error: %v", err)
	}
	if assets != nil {
		t.Errorf("文件不存在时 assets 应为 nil, got %+v", assets)
	}
}

// TestParseClaudeJSONMCPCorrupted 验证:~/.claude.json 存在但损坏时,产出一条
// 带 parse_error 的占位资产(不被静默吞掉)。
func TestParseClaudeJSONMCPCorrupted(t *testing.T) {
	f := newFixture(t)
	f.writeClaudeJSON(`{not valid json`)
	assets, err := parseClaudeJSONMCP(f.cj, ScopeGlobal)
	if err != nil {
		t.Fatalf("损坏文件不应返回 error,应降级为 parse_error 资产: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("want 1 asset, got %d", len(assets))
	}
	a := assets[0]
	if a.Type != AssetMCPServer {
		t.Errorf("want type mcp_server, got %s", a.Type)
	}
	if a.ParseError == "" {
		t.Errorf("缺少 parse_error")
	}
	if a.ID == "" {
		t.Errorf("损坏资产仍需有 ID")
	}
	if a.Hash == "" {
		t.Errorf("损坏资产文件可读,应有 hash")
	}
}

// TestParseClaudeJSONMCPMultipleServers 验证多个 MCP server 均被解析且 ID 唯一。
func TestParseClaudeJSONMCPMultipleServers(t *testing.T) {
	f := newFixture(t)
	f.writeClaudeJSON(`{"mcpServers":{"a":{"command":"cmda"},"b":{"command":"cmdb"}}}`)
	assets, err := parseClaudeJSONMCP(f.cj, ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 2 {
		t.Fatalf("want 2 assets, got %d", len(assets))
	}
	ids := map[string]bool{}
	names := map[string]bool{}
	for _, a := range assets {
		ids[a.ID] = true
		names[a.Name] = true
	}
	if len(ids) != 2 {
		t.Errorf("want 2 个不同 ID, got %d: %v", len(ids), ids)
	}
	if !names["a"] || !names["b"] {
		t.Errorf("两个 server 名应都被解析, got %v", names)
	}
}

// TestDiscoverIncludesClaudeJSONMCP 验证 Discover() 末尾接入 ~/.claude.json MCP 解析,
// 产出的 MCP 资产出现在 inventory 中且 scope 为 global。
func TestDiscoverIncludesClaudeJSONMCP(t *testing.T) {
	f := newFixture(t)
	f.write("settings.json", `{}`)
	f.writeClaudeJSON(`{"mcpServers":{"gmail":{"type":"http","url":"https://x/mcp"}}}`)
	eng := NewEngine(f.home, "")
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}
	var mcps []Asset
	for _, a := range inv.Assets {
		if a.Type == AssetMCPServer {
			mcps = append(mcps, a)
		}
	}
	if len(mcps) != 1 {
		t.Fatalf("want 1 mcp asset in inventory, got %d", len(mcps))
	}
	if mcps[0].Name != "gmail" {
		t.Errorf("want mcp 'gmail', got %s", mcps[0].Name)
	}
	if mcps[0].Scope != ScopeGlobal {
		t.Errorf("mcp scope 应为 global, got %s", mcps[0].Scope)
	}
}
