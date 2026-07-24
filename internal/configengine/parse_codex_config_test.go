package configengine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCodexConfig(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	content := `model = "gpt-5-codex"
approval_policy = "on-failure"
sandbox_mode = "workspace-write"

[mcp_servers.filesystem]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem"]

[mcp_servers.webfetch]
url = "https://example.com/mcp"

[profiles.fast]
model = "gpt-5-codex"
sandbox_mode = "danger-full-access"
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	assets, err := parseCodexConfig(p, ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	// 期望:1 settings + 2 mcp_server + 1 profile = 4
	if len(assets) != 4 {
		t.Fatalf("got %d assets, want 4", len(assets))
	}
	// 找 settings 主体
	var settings *Asset
	mcpCount, profileCount := 0, 0
	for i := range assets {
		switch {
		case assets[i].Type == AssetSettings && assets[i].Name == "config":
			settings = &assets[i]
		case assets[i].Type == AssetMCPServer:
			mcpCount++
		case assets[i].Type == AssetSettings && assets[i].Name == "profile:fast":
			profileCount++
		}
	}
	if settings == nil {
		t.Fatal("缺 settings 主体(Name=config)")
	}
	if settings.Fields["model"] != "gpt-5-codex" {
		t.Fatalf("model = %v", settings.Fields["model"])
	}
	if settings.Fields["approval_policy"] != "on-failure" {
		t.Fatalf("approval_policy = %v", settings.Fields["approval_policy"])
	}
	if settings.Fields["sandbox_mode"] != "workspace-write" {
		t.Fatalf("sandbox_mode = %v", settings.Fields["sandbox_mode"])
	}
	if settings.Fields["raw"] != content {
		t.Fatal("raw 应为整个 config.toml 文本")
	}
	if settings.Content != content {
		t.Fatal("Content 应为整个 config.toml 文本")
	}
	if mcpCount != 2 {
		t.Fatalf("mcp_server 数 = %d, want 2", mcpCount)
	}
	if profileCount != 1 {
		t.Fatalf("profile 数 = %d, want 1", profileCount)
	}
}

func TestParseCodexConfigCorruptTOML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(p, []byte("model = \n this is not valid toml = ="), 0o644); err != nil {
		t.Fatal(err)
	}
	assets, err := parseCodexConfig(p, ScopeGlobal)
	if err != nil {
		t.Fatalf("损坏 TOML 不应返回 error,应产出占位资产: %v", err)
	}
	if len(assets) != 1 || assets[0].ParseError == "" {
		t.Fatal("损坏 TOML 应产出 1 条带 parse_error 的 settings 占位资产")
	}
	if assets[0].Type != AssetSettings {
		t.Fatal("占位资产类型应为 settings")
	}
}

func TestParseCodexConfigMissingFile(t *testing.T) {
	_, err := parseCodexConfig(filepath.Join(t.TempDir(), "nope.toml"), ScopeGlobal)
	if err == nil {
		t.Fatal("文件不存在应返回 error")
	}
}
