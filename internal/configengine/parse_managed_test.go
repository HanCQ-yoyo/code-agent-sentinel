package configengine

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseManagedMCP 验证:parseManagedMCP 解析企业 managed-mcp.json,产出 scope=managed
// 的 mcp_server 资产,Fields["managed"]=true(规则据此识别企业管理模式)。结构同 .mcp.json,
// 但 scope/Fields 标记区分,使基线规则可对 managed server 施加不同策略。
func TestParseManagedMCP(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "managed-mcp.json")
	os.WriteFile(p, []byte(`{
		"mcpServers": {"safe": {"command": "npx", "args": ["-y", "safe"]}}
	}`), 0o644)
	assets, err := parseManagedMCP(p, ScopeManaged)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 {
		t.Fatalf("got %d assets, want 1", len(assets))
	}
	a := assets[0]
	if a.Type != AssetMCPServer {
		t.Fatalf("type = %v, want mcp_server", a.Type)
	}
	if a.Scope != ScopeManaged {
		t.Fatalf("scope = %v, want managed", a.Scope)
	}
	if a.Fields["managed"] != true {
		t.Fatalf("managed 标 = %v, want true", a.Fields["managed"])
	}
	if a.Fields["name"] != "safe" {
		t.Fatalf("name = %v, want safe", a.Fields["name"])
	}
}

// TestParseManagedMCPCorrupt 验证:损坏 JSON 不致全盘失败,产出 1 条带 parse_error 的占位资产
// (与 parseMCPJSON/parseClaudeJSONMCP 损坏分支一致)。文件可读故 hash 应填充。
func TestParseManagedMCPCorrupt(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "managed-mcp.json")
	os.WriteFile(p, []byte(`{not json`), 0o644)
	assets, err := parseManagedMCP(p, ScopeManaged)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].ParseError == "" {
		t.Fatalf("损坏文件应产 1 条带 parse_error 的占位资产, got %+v", assets)
	}
	if assets[0].ID == "" {
		t.Fatal("损坏资产仍需有 ID")
	}
	if assets[0].Hash == "" {
		t.Fatal("损坏资产文件可读,应有 hash")
	}
}
