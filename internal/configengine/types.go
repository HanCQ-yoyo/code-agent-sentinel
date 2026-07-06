package configengine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeProject Scope = "project"
	ScopeManaged Scope = "managed"
	ScopePlugin  Scope = "plugin" // 插件下钻资产(skills/commands/agents/hooks 打包在插件内)
)

type AssetType string

const (
	AssetSettings    AssetType = "settings"
	AssetPermissions AssetType = "permissions"
	AssetHook        AssetType = "hook"
	AssetMCPServer   AssetType = "mcp_server"
	AssetSkill       AssetType = "skill"
	AssetCommand     AssetType = "command"
	AssetAgent       AssetType = "agent"
	AssetPlugin      AssetType = "plugin"
	AssetMemory      AssetType = "memory"
	AssetKeybinding  AssetType = "keybinding"
	AssetScript      AssetType = "script"
)

// Asset 是一个被安全管控的配置项。
type Asset struct {
	ID         string         `json:"id"`
	Type       AssetType      `json:"type"`
	Scope      Scope          `json:"scope"`
	SourcePath string         `json:"source_path"`
	Name       string         `json:"name"`
	Fields     map[string]any `json:"fields,omitempty"`
	Content    string         `json:"content,omitempty"`
	MTime      time.Time      `json:"mtime"`
	Hash       string         `json:"hash"`
	ParseError string         `json:"parse_error,omitempty"`
}

// makeAssetID 返回稳定标识(scope:type:name:path 的 sha256 前 16 字节)。
// 注意:不能定义同名方法 ID(),Go 不允许字段与方法同名,故用独立函数。
func makeAssetID(a Asset) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s:%s", a.Scope, a.Type, a.Name, a.SourcePath)))
	return hex.EncodeToString(h[:16])
}

// Project 是一个可被切换的代码项目。
type Project struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// Inventory 是一次发现的全部资产。
type Inventory struct {
	Assets     []Asset     `json:"assets"`
	Project    *Project    `json:"project,omitempty"`
	Duplicates []Duplicate `json:"duplicates,omitempty"`
}

// Filter 按类型/范围过滤。
func (inv Inventory) Filter(typ AssetType, scope Scope) []Asset {
	var out []Asset
	for _, a := range inv.Assets {
		if (typ == "" || a.Type == typ) && (scope == "" || a.Scope == scope) {
			out = append(out, a)
		}
	}
	return out
}
