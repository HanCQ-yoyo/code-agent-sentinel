package configengine

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

// Discover 发现全局资产:settings 单文件 + skills/commands/agents markdown 目录 +
// ~/.claude.json 的 mcpServers + memory(CLAUDE.md + memory/)+ plugins +
// keybindings + scripts(从 hook/command 抽取)。
// 项目级发现在 discover_project.go(Task 8)。
func (e *Engine) Discover() (Inventory, error) {
	inv := Inventory{}
	claude := filepath.Join(e.HomeDir, ".claude")

	// settings.json:真实解析,拆成 settings + permissions + 每个 hook 一条。
	// settings.local.json:本地覆盖层(Claude Code 在此写 project-scoped 覆盖,
	// 与 settings.json 同结构,优先级更高)。两者都发现,Name 区分(settings /
	// settings.local)以避免 ID 冲突。CLAUDE.md 不在此处处理,改由 parseMemory 覆盖。
	for _, sf := range []string{"settings.json", "settings.local.json"} {
		if p := filepath.Join(claude, sf); fileExists(p) {
			parsed, _ := parseSettings(p, ScopeGlobal)
			inv.Assets = append(inv.Assets, parsed...)
		}
	}

	// 目录资产:skills/commands/agents,每个含 .md 的顶层条目产出一条资产。
	for _, d := range []struct {
		rel string
		typ AssetType
	}{
		{"skills", AssetSkill},
		{"commands", AssetCommand},
		{"agents", AssetAgent},
	} {
		if assets, _ := parseMarkdownDir(filepath.Join(claude, d.rel), d.typ, ScopeGlobal); assets != nil {
			inv.Assets = append(inv.Assets, assets...)
		}
	}

	// ~/.claude.json 顶层 mcpServers(机器管理文件,只读)。文件可能不存在,
	// parseClaudeJSONMCP 在缺失时返回 nil, nil,故可无条件调用;损坏文件会
	// 产出带 parse_error 的占位资产,不被静默吞掉。
	if mcpAssets, err := parseClaudeJSONMCP(e.ClaudeJSON, ScopeGlobal); err == nil {
		inv.Assets = append(inv.Assets, mcpAssets...)
	}

	// memory:CLAUDE.md + memory/ 目录(含真实内容 hash)。
	if mem, _ := parseMemory(claude, ScopeGlobal); mem != nil {
		inv.Assets = append(inv.Assets, mem...)
	}

	// plugins:遍历 plugins/cache/<marketplace>/<plugin>/。
	if pl, _ := parsePlugins(claude, ScopeGlobal); pl != nil {
		inv.Assets = append(inv.Assets, pl...)
	}

	// keybindings.json:键→动作映射(Task 7 起真实解析,取代旧 single 占位)。
	if kb, _ := parseKeybindings(filepath.Join(claude, "keybindings.json"), ScopeGlobal); kb != nil {
		inv.Assets = append(inv.Assets, kb...)
	}

	// scripts:在所有解析完成后,从 hook/command 资产的 command 字段抽取引用脚本。
	inv.Assets = append(inv.Assets, parseScripts(inv.Assets, claude)...)

	// 项目级发现:遍历所有已知项目(全 agent 发现),合并各项目的资产。
	// discoverProjects 内部对项目 scope 的 hook/command 单独跑 parseScripts,避免与
	// 上面的全局 parseScripts 重复(见 discover_project.go 偏差注释)。
	e.discoverProjects(&inv)

	// 重复检测:跨 scope / 跨 source_path 的同类型同名资产。
	inv.Duplicates = detectDuplicates(inv.Assets)
	return inv, nil
}

// placeholder 产出一个仅含 hash/mtime 的占位资产(解析任务会填充 Fields/Content)。
//
// 说明:brief 的 placeholder 直接调用 HashAndMTime(path),但目录资产
// (skills/commands/agents 的顶层子目录)在 Linux 上无法用 io.Copy 读取内容
// (os.Open 成功但 io.Copy 报 "is a directory"),导致 Hash 为空、测试失败。
// 此处对目录用 stat mtime 产出存在性指纹(sha256("dir:"+path) 前 16 字节),
// 不修改 Task 2 的 HashAndMTime。
//
// 保留:Task 7 起 Discover 不再调用此方法(settings/keybindings/CLAUDE.md 均已
// 真实解析),Task 8 项目级发现也直接用 parse* 函数而非此占位。保留方法本身
// 供后续可能的目录资产占位场景复用;Go 不对未使用的方法报错,留着无害。
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
