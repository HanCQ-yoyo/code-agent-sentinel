package configengine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHashAndMTime(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, mt, err := HashAndMTime(p)
	if err != nil {
		t.Fatal(err)
	}
	if hash != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("bad hash: %s", hash)
	}
	if mt.IsZero() || time.Since(mt) > time.Minute {
		t.Fatalf("bad mtime: %v", mt)
	}
}

func TestAssetIDStable(t *testing.T) {
	a := Asset{Type: AssetMCPServer, Scope: ScopeGlobal, Name: "foo", SourcePath: "/p"}
	id1 := makeAssetID(a)
	id2 := makeAssetID(Asset{Type: AssetMCPServer, Scope: ScopeGlobal, Name: "foo", SourcePath: "/p"})
	if id1 == "" || id1 != id2 {
		t.Fatal("ID 不稳定")
	}
}

func TestScopePluginConstant(t *testing.T) {
	// ScopePlugin 用于插件下钻资产,Filter 须能按之过滤。
	if ScopePlugin != "plugin" {
		t.Fatalf("ScopePlugin = %q, want %q", ScopePlugin, "plugin")
	}
	inv := Inventory{Assets: []Asset{
		{Type: AssetSkill, Scope: ScopePlugin, Name: "brainstorming"},
		{Type: AssetSkill, Scope: ScopeGlobal, Name: "global-skill"},
	}}
	got := inv.Filter("", ScopePlugin)
	if len(got) != 1 || got[0].Name != "brainstorming" {
		t.Fatalf("Filter ScopePlugin = %+v, want only brainstorming", got)
	}
}
