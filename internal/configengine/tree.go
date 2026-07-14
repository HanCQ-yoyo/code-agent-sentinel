package configengine

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TreeNode 是目录树的一个节点:目录(dir)、文件(file,可挂多资产)、或 synthetic(根外资产分组)。
type TreeNode struct {
	Name     string     `json:"name"`                // 显示名(目录名/文件名/synthetic 名)
	Path     string     `json:"path"`                // 相对 root 的路径
	Kind     string     `json:"kind"`                // "dir" | "file" | "synthetic"
	Scope    string     `json:"scope,omitempty"`     // 该节点资产的 scope(标记色用);目录/无资产则空
	AssetIDs []string   `json:"asset_ids,omitempty"` // 挂在此节点的资产 ID(同路径多资产合并)
	Children []TreeNode `json:"children,omitempty"`
}

// treeNode 是内部的指针节点,用于 BuildTree 构建阶段。
//
// 为什么需要指针节点:brief 的起始实现把 *值* 追加到 []TreeNode 后取
// &children[idx] 存入 byPath 索引。但 build 循环会持续 append 到同一
// children slice,append 一旦超容就会重新分配底层数组 → 之前存进 byPath
// 的指针指向旧数组,成为悬挂指针。挂资产阶段通过 byPath 查到的节点地址
// 已失效,写入要么落空要么写到不可见内存。用 *treeNode(每个节点独立
// &treeNode{...} 分配)保证地址不随 slice 增长失效;最后统一解引用成
// 公开的值类型 TreeNode 树。
type treeNode struct {
	Name     string
	Path     string
	Kind     string
	Scope    string
	AssetIDs []string
	Children []*treeNode
}

// toValue 将内部指针节点递归转为公开的值类型节点。
func (n *treeNode) toValue() TreeNode {
	v := TreeNode{
		Name:     n.Name,
		Path:     n.Path,
		Kind:     n.Kind,
		Scope:    n.Scope,
		AssetIDs: n.AssetIDs,
	}
	if len(n.Children) > 0 {
		v.Children = make([]TreeNode, len(n.Children))
		for i, c := range n.Children {
			v.Children[i] = c.toValue()
		}
	}
	return v
}

// BuildTree 真实走 root 文件系统建目录树,并按 source_path 把 assets 挂到对应 file 节点。
// 根外资产(source_path 不在 root 下)收进根级 synthetic 节点。
// root 不存在/不可读返回 error;单个子目录不可读跳过,不整体失败。
func (e *Engine) BuildTree(root string, assets []Asset) (TreeNode, error) {
	// 预检 root 存在且是目录。
	fi, err := os.Stat(root)
	if err != nil || !fi.IsDir() {
		return TreeNode{}, os.ErrNotExist
	}
	// 预检 root 可读:root 不可读要返回 error(区别于子目录不可读只跳过)。
	// 放在 build 之外,使 build 闭包无需 error 返回——子目录不可读静默跳过更诚实。
	if _, err := os.ReadDir(root); err != nil {
		return TreeNode{}, err
	}

	// 1) 真实走 fs 建 dir/file 树。索引:相对路径 → *treeNode(指针稳定,便于挂资产)。
	byPath := map[string]*treeNode{}
	var build func(absDir string, rel string) []*treeNode
	build = func(absDir string, rel string) []*treeNode {
		entries, err := os.ReadDir(absDir)
		if err != nil {
			// 子目录不可读:跳过该子树,返回空(不整体失败)。
			return nil
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		var children []*treeNode
		for _, en := range entries {
			childRel := filepath.Join(rel, en.Name())
			node := &treeNode{Name: en.Name(), Path: childRel}
			if en.IsDir() {
				node.Kind = "dir"
				node.Children = build(filepath.Join(absDir, en.Name()), childRel)
			} else {
				node.Kind = "file"
			}
			// 指针节点:append 到 slice 的也是指针(8 字节拷贝),byPath 存的
			// 指针指向独立分配的 treeNode,不依赖 children 底层数组地址。
			children = append(children, node)
			byPath[childRel] = node
		}
		return children
	}
	rootChildren := build(root, ".")
	rootNode := &treeNode{Name: filepath.Base(root), Path: ".", Kind: "dir", Children: rootChildren}

	// 2) 挂资产:把 source_path 转相对 root;在 root 内的挂到 file 节点,根外的进 synthetic。
	type synGroup struct {
		ids   []string
		scope string
	}
	synthetic := map[string]*synGroup{}
	for _, a := range assets {
		rel, err := filepath.Rel(root, a.SourcePath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
			// 根外资产:synthetic 分组(按 basename)。
			base := filepath.Base(a.SourcePath)
			g := synthetic[base]
			if g == nil {
				g = &synGroup{}
				synthetic[base] = g
			}
			g.ids = append(g.ids, a.ID)
			g.scope = string(a.Scope)
			continue
		}
		// 规范化:filepath.Rel 对 root 本身返回 ".";对 root 内文件返回相对路径。
		// 去掉首部 "./" 以匹配 byPath 的 key(无前缀)。
		key := strings.TrimPrefix(rel, "./")
		if key == "." {
			// 极少:资产 source_path 就是 root 本身;无对应 file 节点,跳过。
			continue
		}
		node := byPath[key]
		if node == nil {
			// 资产路径在 root 内但文件系统无对应条目(例如目录型资产 source_path
			// 指向一个目录,而该目录在树里是某 file 节点的父级——不应丢):退化为
			// synthetic 分组(按 basename),保证资产不丢。
			base := filepath.Base(a.SourcePath)
			g := synthetic[base]
			if g == nil {
				g = &synGroup{}
				synthetic[base] = g
			}
			g.ids = append(g.ids, a.ID)
			g.scope = string(a.Scope)
			continue
		}
		node.AssetIDs = append(node.AssetIDs, a.ID)
		node.Scope = string(a.Scope)
	}

	// 3) synthetic 节点追加到根(按 name 排序,与目录节点一致)。
	var synKeys []string
	for k := range synthetic {
		synKeys = append(synKeys, k)
	}
	sort.Strings(synKeys)
	for _, k := range synKeys {
		rootNode.Children = append(rootNode.Children, &treeNode{
			Name: k, Path: k, Kind: "synthetic", Scope: synthetic[k].scope, AssetIDs: synthetic[k].ids,
		})
	}

	return rootNode.toValue(), nil
}

// BuildTreeFromAssets 构造一棵仅由资产驱动的树:不读真实文件系统,把 assets 按
// source_path 分组为根级 file/synthetic 节点。用于项目根目录缺失(如项目仅有
// 根级 .mcp.json 而无 .claude/ 子目录)时 BuildTree 会因 root 不存在返回 error 的场景——
// 此处降级为只展示资产,保证前端文件树仍可见该项目的资产,而非 500 白屏。
//
// 语义与 BuildTree 的 synthetic 分组一致:同 basename 的多资产合并到一个节点。
// 排序按 basename 字典序,与 BuildTree 的 synthetic 追加顺序一致。
func (e *Engine) BuildTreeFromAssets(root string, assets []Asset) TreeNode {
	type group struct {
		ids   []string
		scope string
	}
	groups := map[string]*group{}
	for _, a := range assets {
		base := filepath.Base(a.SourcePath)
		g := groups[base]
		if g == nil {
			g = &group{}
			groups[base] = g
		}
		g.ids = append(g.ids, a.ID)
		g.scope = string(a.Scope)
	}
	var keys []string
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	children := make([]TreeNode, 0, len(keys))
	for _, k := range keys {
		// file 节点(有资产挂载);Path 用 basename,与 BuildTree synthetic 节点 Path 一致,
		// 前端 byPath 索引按 path 查节点不受影响。
		children = append(children, TreeNode{
			Name: k, Path: k, Kind: "file", Scope: groups[k].scope, AssetIDs: groups[k].ids,
		})
	}
	return TreeNode{
		Name:     filepath.Base(root),
		Path:     ".",
		Kind:     "dir",
		Children: children,
	}
}
