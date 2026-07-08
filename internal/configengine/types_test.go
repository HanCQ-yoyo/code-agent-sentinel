package configengine

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestInventoryProjectsField(t *testing.T) {
	inv := Inventory{Projects: []Project{{Path: "/a", Name: "a"}, {Path: "/b", Name: "b"}}}
	if len(inv.Projects) != 2 {
		t.Fatalf("Projects 长度 = %d, 期望 2", len(inv.Projects))
	}
	if inv.Projects[0].Path != "/a" {
		t.Errorf("Projects[0].Path = %q", inv.Projects[0].Path)
	}
}

func TestEngineHasNoSelectProject(t *testing.T) {
	// SelectProject 已删除:若残留方法,以下断言失败。
	var e *Engine
	_, ok := reflect.TypeOf(e).MethodByName("SelectProject")
	if ok {
		t.Fatal("Engine 不应再有 SelectProject 方法(本任务已移除单项目选择)")
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
