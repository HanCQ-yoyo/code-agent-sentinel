package configengine

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

// Discover 发现全局 + 当前项目的资产(本任务仅全局枚举占位)。
func (e *Engine) Discover() (Inventory, error) {
	inv := Inventory{Project: e.Project}
	claude := filepath.Join(e.HomeDir, ".claude")

	// 单文件资产(占位:仅记录存在性 + hash,解析在后续任务)。
	single := []struct {
		rel  string
		typ  AssetType
		name string
	}{
		{"settings.json", AssetSettings, "settings"},
		{"keybindings.json", AssetKeybinding, "keybindings"},
		{"CLAUDE.md", AssetMemory, "CLAUDE.md"},
	}
	for _, s := range single {
		p := filepath.Join(claude, s.rel)
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if s.typ == AssetSettings {
			// 真实解析:settings.json 拆成 settings + permissions + 每个 hook 一条。
			parsed, _ := parseSettings(p, ScopeGlobal)
			inv.Assets = append(inv.Assets, parsed...)
			continue
		}
		inv.Assets = append(inv.Assets, e.placeholder(p, s.typ, ScopeGlobal, s.name))
	}

	// 目录资产:skills/commands/agents(占位,每个顶层条目一条)。
	for _, d := range []struct{ rel, typ string }{
		{"skills", string(AssetSkill)},
		{"commands", string(AssetCommand)},
		{"agents", string(AssetAgent)},
	} {
		base := filepath.Join(claude, d.rel)
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, en := range entries {
			if !en.IsDir() {
				continue
			}
			inv.Assets = append(inv.Assets, e.placeholder(filepath.Join(base, en.Name()), AssetType(d.typ), ScopeGlobal, en.Name()))
		}
	}

	// ~/.claude.json 顶层 mcpServers(机器管理文件,只读)。文件可能不存在,
	// parseClaudeJSONMCP 在缺失时返回 nil, nil,故可无条件调用;损坏文件会
	// 产出带 parse_error 的占位资产,不被静默吞掉。
	if mcpAssets, err := parseClaudeJSONMCP(e.ClaudeJSON, ScopeGlobal); err == nil {
		inv.Assets = append(inv.Assets, mcpAssets...)
	}
	return inv, nil
}

// placeholder 产出一个仅含 hash/mtime 的占位资产(解析任务会填充 Fields/Content)。
//
// 说明:brief 的 placeholder 直接调用 HashAndMTime(path),但目录资产
// (skills/commands/agents 的顶层子目录)在 Linux 上无法用 io.Copy 读取内容
// (os.Open 成功但 io.Copy 报 "is a directory"),导致 Hash 为空、测试失败。
// 此处对目录用 stat mtime 产出存在性指纹(sha256("dir:"+path) 前 16 字节),
// 不修改 Task 2 的 HashAndMTime。Tasks 4-7 解析时会用真实文件内容覆盖。
func (e *Engine) placeholder(path string, typ AssetType, scope Scope, name string) Asset {
	a := Asset{Type: typ, Scope: scope, SourcePath: path, Name: name}
	if h, mt, err := HashAndMTime(path); err == nil {
		a.Hash, a.MTime = h, mt
	} else if fi, statErr := os.Stat(path); statErr == nil && fi.IsDir() {
		// 目录无法内容 hash;用 stat mtime + 路径指纹作存在性标记。
		a.MTime = fi.ModTime()
		dh := sha256.Sum256([]byte("dir:" + path))
		a.Hash = hex.EncodeToString(dh[:16])
	} else {
		a.ParseError = err.Error()
	}
	a.ID = makeAssetID(a)
	return a
}

// readProjectList 占位,Task 8 在 discover_project.go 实现真实版本并删除此 stub。
func readProjectList(claudeJSON string) ([]Project, error) { return nil, nil }
