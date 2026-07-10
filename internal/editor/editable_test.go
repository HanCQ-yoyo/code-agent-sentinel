package editor

import (
	"os"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

// newFixture 造一个临时 home + ~/.claude,返回 (home, claudeDir)。
func newFixture(t *testing.T) (home, claude string) {
	t.Helper()
	home = t.TempDir()
	claude = filepath.Join(home, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	return home, claude
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEditableGlobalSettings(t *testing.T) {
	home, claude := newFixture(t)
	writeFile(t, filepath.Join(claude, "settings.json"), `{"model":"opus"}`)
	e := New(configengine.NewEngine(home), "", 0)
	inv, err := e.Engine.Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Assets) == 0 {
		t.Fatal("no assets")
	}
	ok, reason := e.editable(inv.Assets[0])
	if !ok {
		t.Fatalf("global settings should be editable: %s", reason)
	}
}

func TestEditableRejectsClaudeJSON(t *testing.T) {
	home, claude := newFixture(t)
	// ~/.claude.json 全局 MCP(机器管理,只读)
	writeFile(t, filepath.Join(home, ".claude.json"), `{"mcpServers":{"foo":{"command":"bar"}}}`)
	writeFile(t, filepath.Join(claude, "settings.json"), `{"model":"opus"}`)
	e := New(configengine.NewEngine(home), "", 0)
	inv, err := e.Engine.Discover()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range inv.Assets {
		if a.Type == configengine.AssetMCPServer && a.SourcePath == filepath.Join(home, ".claude.json") {
			ok, _ := e.editable(a)
			if ok {
				t.Fatal("MCP from ~/.claude.json must not be editable")
			}
			return
		}
	}
	t.Fatal("test setup: no global MCP asset found")
}

func TestEditableRejectsOutOfRoot(t *testing.T) {
	home, _ := newFixture(t)
	e := New(configengine.NewEngine(home), "", 0)
	// 伪造一个指向 /etc 的资产
	rogue := configengine.Asset{
		Type:       configengine.AssetSettings,
		Scope:      configengine.ScopeGlobal,
		SourcePath: "/etc/passwd",
	}
	ok, _ := e.editable(rogue)
	if ok {
		t.Fatal("out-of-root asset must not be editable")
	}
}

func TestEditableProjectRequiresKnownProject(t *testing.T) {
	home, _ := newFixture(t)
	// ~/.claude.json 登记一个项目,使 ListProjects 含它
	projDir := filepath.Join(home, "myproj")
	writeFile(t, filepath.Join(projDir, ".claude", "settings.json"), `{"model":"opus"}`)
	writeFile(t, filepath.Join(home, ".claude.json"),
		`{"projects":{"`+projDir+`":{}}}`)
	e := New(configengine.NewEngine(home), "", 0)
	inv, err := e.Engine.Discover()
	if err != nil {
		t.Fatal(err)
	}
	var projAsset *configengine.Asset
	for i := range inv.Assets {
		if inv.Assets[i].Scope == configengine.ScopeProject {
			projAsset = &inv.Assets[i]
			break
		}
	}
	if projAsset == nil {
		t.Fatal("no project asset")
	}
	ok, _ := e.editable(*projAsset)
	if !ok {
		t.Fatal("known project asset should be editable")
	}
	// 未知项目路径
	rogue := configengine.Asset{
		Type:       configengine.AssetSettings,
		Scope:      configengine.ScopeProject,
		SourcePath: filepath.Join(home, "unknown-proj", ".claude", "settings.json"),
	}
	ok, _ = e.editable(rogue)
	if ok {
		t.Fatal("unknown project asset must not be editable")
	}
}

func TestFindAssetByID(t *testing.T) {
	home, claude := newFixture(t)
	writeFile(t, filepath.Join(claude, "settings.json"), `{"model":"opus"}`)
	e := New(configengine.NewEngine(home), "", 0)
	inv, _ := e.Engine.Discover()
	want := inv.Assets[0]
	got, ok := e.findAsset(want.ID)
	if !ok {
		t.Fatal("findAsset missed existing asset")
	}
	if got.ID != want.ID {
		t.Fatalf("findAsset got %q want %q", got.ID, want.ID)
	}
	_, ok = e.findAsset("nonexistent")
	if ok {
		t.Fatal("findAsset should miss nonexistent id")
	}
}
