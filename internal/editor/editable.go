package editor

import (
	"os"
	"path/filepath"
	"strings"

	"code-agent-sentinel/internal/configengine"
)

// editableAssetTypes 是可编辑资产类型白名单。
// ~/.claude.json(全局MCP+项目状态)、plugins cache、managed 只读,不在此列或由路径规则拒绝。
var editableAssetTypes = map[configengine.AssetType]bool{
	configengine.AssetSettings:    true,
	configengine.AssetPermissions: true,
	configengine.AssetHook:        true,
	configengine.AssetMCPServer:   true, // 仅 .mcp.json 来源;~/.claude.json 来源由路径规则拒绝
	configengine.AssetSkill:       true,
	configengine.AssetCommand:     true,
	configengine.AssetAgent:       true,
	configengine.AssetMemory:      true,
	configengine.AssetKeybinding:  true,
	configengine.AssetScript:      true,
}

// findAsset 按 ID 在最新 Discover 结果中查资产。
func (e *Editor) findAsset(id string) (configengine.Asset, bool) {
	inv, err := e.Engine.Discover()
	if err != nil {
		return configengine.Asset{}, false
	}
	for _, a := range inv.Assets {
		if a.ID == id {
			return a, true
		}
	}
	return configengine.Asset{}, false
}

// editable 判定资产是否可编辑,返回 (ok, reason)。reason 非空时说明拒绝原因。
func (e *Editor) editable(a configengine.Asset) (bool, string) {
	if !editableAssetTypes[a.Type] {
		return false, "asset type not editable"
	}
	sp := a.SourcePath
	if sp == "" {
		return false, "empty source path"
	}
	// 拒绝 ~/.claude.json(机器管理:全局 MCP + projects 状态)
	if sp == e.Engine.ClaudeJSON {
		return false, "~/.claude.json is machine-managed (read-only)"
	}
	// 拒绝 plugins cache(第三方,只读)
	if strings.Contains(sp, string(filepath.Separator)+"plugins"+string(filepath.Separator)+"cache"+string(filepath.Separator)) {
		return false, "plugins cache is third-party (read-only)"
	}
	// 合法根校验:全局 ~/.claude 或 已知项目 <p>/.claude
	claudeDir := filepath.Join(e.Engine.HomeDir, ".claude")
	switch a.Scope {
	case configengine.ScopeGlobal, configengine.ScopeManaged:
		// managed 应只读;global 须在 ~/.claude 下
		if a.Scope == configengine.ScopeManaged {
			return false, "managed policy is read-only"
		}
		if !pathInDir(sp, claudeDir) {
			return false, "global asset out of ~/.claude"
		}
	case configengine.ScopeProject:
		ok, root := e.projectRootFor(sp)
		if !ok {
			return false, "project not known or path out of project .claude"
		}
		_ = root
	case configengine.ScopePlugin:
		return false, "plugin assets are read-only"
	default:
		return false, "unknown scope"
	}
	// symlink 不下钻:目标须是真实文件(os.Lstat 非 symlink)
	if isSymlink(sp) {
		return false, "symlink targets not editable"
	}
	return true, ""
}

// projectRootFor 校验 project scope 资产:SourcePath 须落在某已知项目 <p>/.claude 下。
// 返回 (ok, <p>/.claude)。项目须 ListProjects 精确 == 匹配(非前缀/包含)。
func (e *Editor) projectRootFor(sp string) (bool, string) {
	known, _ := e.Engine.ListProjects()
	for _, p := range known {
		root := filepath.Join(p.Path, ".claude")
		if pathInDir(sp, root) {
			return true, root
		}
	}
	return false, ""
}

// pathInDir 报告 path 是否在 dir 之下(严格子路径,用 filepath.Rel 防 ../ 逃逸)。
func pathInDir(path, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil {
		return false
	}
	if rel == "." {
		return false // 资产是文件不应等于目录
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// isSymlink 报告 path 是否是 symlink(Lstat;若文件不存在也返 false,后续读写自然报错)。
func isSymlink(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSymlink != 0
}
