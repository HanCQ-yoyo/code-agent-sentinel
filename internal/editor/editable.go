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
	// 合法根校验:编辑器只能编辑 configengine 发现的资产(findAsset → Discover),
	// 而 configengine 在 home / 项目根下发现脚本与 .mcp.json(不止 .claude/):
	//   - 全局:parseScripts 以 ~/.claude 的父目录(home)为 base,故全局 hook 引用
	//     的脚本可能落在 ~/scripts/...(home 下、.claude 外)。
	//   - 项目:.mcp.json 直接在 <p>/ 下(discover_project.go),项目 hook 引用的
	//     脚本以 <p>/.claude 的父目录(<p>)为 base,落在 <p>/scripts/...。
	// 故合法根为 home(全局)/ <p>(项目);~/.claude.json 与 plugins cache 检查仍是
	// 机器管理/第三方文件的守门员。fail-closed:未知根一律拒绝。
	var root string
	switch a.Scope {
	case configengine.ScopeGlobal, configengine.ScopeManaged:
		// managed 应只读;global 须在 home 下
		if a.Scope == configengine.ScopeManaged {
			return false, "managed policy is read-only"
		}
		if !pathInDir(sp, e.Engine.HomeDir) {
			return false, "global asset out of home"
		}
		root = e.Engine.HomeDir
	case configengine.ScopeProject:
		ok, r := e.projectRootFor(sp)
		if !ok {
			return false, "project not known or path out of project root"
		}
		root = r
	case configengine.ScopePlugin:
		return false, "plugin assets are read-only"
	default:
		return false, "unknown scope"
	}
	// symlink 不下钻:目标须是真实文件(os.Lstat 非 symlink)
	//
	// 注意:editable() 是 snapshot 判定,实际 os.ReadFile/os.Rename 在 Commit 中稍后发生
	// (见 editor.go)。该时间窗内若有人新建符号链接,理论上可绕过此处校验(TOCTOU)。
	// 这是本地单用户工具的有意权衡,非安全边界:本地用户本就有任意写权限,
	// 编辑器只做防呆而非防对抗。若未来引入多用户/远程,须在写盘前重做该校验。
	if isSymlink(sp) {
		return false, "symlink targets not editable"
	}
	// 父目录符号链接防护:isSymlink 只检查叶子节点,若父目录(如 ~/.claude/scripts)
	// 是指向 root 之外的 symlink,os.ReadFile/os.Rename 会解析符号链接写入 root 之外。
	// EvalSymlinks 解析路径上的所有符号链接(含父目录),再用真实路径重新校验 root。
	resolved, err := filepath.EvalSymlinks(sp)
	if err != nil {
		return false, "path resolve failed"
	}
	if !pathInDir(resolved, root) {
		return false, "out of root (symlink resolved)"
	}
	return true, ""
}

// projectRootFor 校验 project scope 资产:SourcePath 须落在某已知项目 <p> 下。
// 返回 (ok, <p>)。项目须 ListProjects 精确 == 匹配(非前缀/包含)。
// 合法根是项目根 <p>(非 <p>/.claude),因为 configengine 在 <p>/ 下发现 .mcp.json
// 与 <p>/scripts/...(parseScripts base = <p>/.claude 的父目录 = <p>)。
func (e *Editor) projectRootFor(sp string) (bool, string) {
	known, _ := e.Engine.ListProjects()
	for _, p := range known {
		if pathInDir(sp, p.Path) {
			return true, p.Path
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
