package configengine

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// findNode 在 root 树里按路径段定位子节点;返回 nil 表示未找到。
func findNode(t *testing.T, root TreeNode, path string) *TreeNode {
	t.Helper()
	if root.Path == path {
		return &root
	}
	var walk func(TreeNode) *TreeNode
	walk = func(n TreeNode) *TreeNode {
		for i := range n.Children {
			c := &n.Children[i]
			if c.Path == path {
				return c
			}
			if r := walk(*c); r != nil {
				return r
			}
		}
		return nil
	}
	return walk(root)
}

func TestBuildTreeRealDirsAndMergedAssets(t *testing.T) {
	f := newFixture(t)
	// 真实目录:settings.json(单文件)、skills/injection/SKILL.md、empty/(空目录)
	f.write("settings.json", `{"model":"opus"}`)
	f.write("skills/injection/SKILL.md", `# skill`)

	eng := NewEngine(f.home)
	inv, err := eng.Discover()
	if err != nil {
		t.Fatal(err)
	}
	root, err := eng.BuildTree(f.claude, inv.Assets)
	if err != nil {
		t.Fatalf("BuildTree: %v", err)
	}
	// settings.json 节点应合并 settings + permissions(+ 可能的 hook)资产
	sn := findNode(t, root, "settings.json")
	if sn == nil {
		t.Fatal("未找到 settings.json 节点")
	}
	if sn.Kind != "file" {
		t.Errorf("settings.json Kind = %q, 期望 file", sn.Kind)
	}
	if len(sn.AssetIDs) < 1 {
		t.Errorf("settings.json 应至少挂 1 个资产,实际 %d", len(sn.AssetIDs))
	}
	// skills/injection/SKILL.md 真实下钻到两层
	if findNode(t, root, "skills") == nil {
		t.Error("缺 skills 目录节点")
	}
	if findNode(t, root, filepath.Join("skills", "injection", "SKILL.md")) == nil {
		t.Error("缺 skills/injection/SKILL.md 文件节点")
	}
}

func TestBuildTreeEmptyDirVisible(t *testing.T) {
	f := newFixture(t)
	f.write("settings.json", `{}`)
	// 造一个空目录(无资产、无文件)
	if err := os.Mkdir(filepath.Join(f.claude, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	eng := NewEngine(f.home)
	inv, _ := eng.Discover()
	root, err := eng.BuildTree(f.claude, inv.Assets)
	if err != nil {
		t.Fatalf("BuildTree: %v", err)
	}
	en := findNode(t, root, "empty")
	if en == nil {
		t.Fatal("空目录应作为节点出现")
	}
	if en.Kind != "dir" {
		t.Errorf("empty Kind = %q, 期望 dir", en.Kind)
	}
	if len(en.Children) != 0 {
		t.Errorf("空目录应无 children,实际 %d", len(en.Children))
	}
}

func TestBuildTreeSyntheticForOutsideAssets(t *testing.T) {
	f := newFixture(t)
	f.write("settings.json", `{}`)
	// .claude.json 在 home 下(root 之外),发产出 scope=global 的 mcp_server 资产
	f.writeClaudeJSON(`{"mcpServers":{"s1":{"command":"x"}}}`)
	// 外部脚本(root 之外):settings hook 引用 home 下的 foo.sh
	fooSh := filepath.Join(f.home, "foo.sh")
	f.write("../foo.sh", "#!/bin/bash\necho hi\n") // write 的 rel 是相对 .claude
	f.write("settings.json", `{"hooks":{"PreToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"bash `+fooSh+`"}]}]}}`)

	eng := NewEngine(f.home)
	inv, _ := eng.Discover()
	root, err := eng.BuildTree(f.claude, inv.Assets)
	if err != nil {
		t.Fatalf("BuildTree: %v", err)
	}
	// 根外资产(.claude.json / foo.sh)应进 synthetic 节点
	var synNames []string
	for _, c := range root.Children {
		if c.Kind == "synthetic" {
			synNames = append(synNames, c.Path)
		}
	}
	sort.Strings(synNames)
	hasClaudeJSON := false
	for _, n := range synNames {
		if n == ".claude.json" {
			hasClaudeJSON = true
		}
	}
	if !hasClaudeJSON {
		t.Errorf("根外资产 .claude.json 应进 synthetic 节点;synthetic=%v", synNames)
	}
}

func TestBuildTreePluginsRealDrill(t *testing.T) {
	f := newFixture(t)
	f.write("settings.json", `{}`)
	// 造一个插件目录结构:plugins/cache/mkt/plug/0.0.1/.claude-plugin/plugin.json + skills/x/SKILL.md
	f.write("plugins/cache/mkt/plug/0.0.1/.claude-plugin/plugin.json", `{"name":"plug","version":"0.0.1"}`)
	f.write("plugins/cache/mkt/plug/0.0.1/skills/x/SKILL.md", `# plug skill`)

	eng := NewEngine(f.home)
	inv, _ := eng.Discover()
	root, err := eng.BuildTree(f.claude, inv.Assets)
	if err != nil {
		t.Fatalf("BuildTree: %v", err)
	}
	// plugins 真实下钻到 cache/mkt/plug/0.0.1
	if findNode(t, root, filepath.Join("plugins", "cache")) == nil {
		t.Error("缺 plugins/cache 节点(应真实下钻)")
	}
}

func TestBuildTreeRootMissing(t *testing.T) {
	eng := NewEngine("/nonexistent-home-xyz")
	_, err := eng.BuildTree(filepath.Join("/nonexistent-home-xyz", ".claude"), nil)
	if err == nil {
		t.Error("root 不存在时应返回 error")
	}
}
