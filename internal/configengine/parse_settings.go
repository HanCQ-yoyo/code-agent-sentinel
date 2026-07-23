package configengine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type rawHook struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type rawHookEntry struct {
	Matcher string    `json:"matcher"`
	Hooks   []rawHook `json:"hooks"`
}

type rawSettings struct {
	Model       string            `json:"model"`
	Env         map[string]string `json:"env"`
	Permissions struct {
		Allow []string `json:"allow"`
		Deny  []string `json:"deny"`
		Ask   []string `json:"ask"`
	} `json:"permissions"`
	Hooks map[string][]rawHookEntry `json:"hooks"`
	// 其余字段未单独建模;通过 base.Fields["raw"] 保留原始 JSON 供后续检测器使用。
}

// parseSettings 解析 settings.json / settings.local.json,产出 settings + permissions +
// 每个 hook 一条资产。
//
// baseName 按源文件名推导(settings.json→"settings",settings.local.json→"settings.local"),
// 使本地覆盖层资产有独立 Name,避免与 settings.json 同 scope 同 source_path 同名导致 ID
// 冲突(被去重静默丢弃),并在树视图里成为独立文件节点。
//
// 损坏文件不致失败:返回一条带 parse_error 的 settings 占位资产,供上层作为 Finding 暴露。
func parseSettings(path string, scope Scope) ([]Asset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	baseName := settingsBaseName(path)
	var rs rawSettings
	if err := json.Unmarshal(data, &rs); err != nil {
		// 损坏文件:产出一条带 parse_error 的 settings 占位资产。
		// 文件本身可读,故仍填 hash/mtime(与 placeholder 行为一致);fillHash 内部会设 ID。
		a := Asset{Type: AssetSettings, Scope: scope, SourcePath: path, Name: baseName, ParseError: err.Error()}
		fillHash(&a)
		return []Asset{a}, nil
	}
	var out []Asset

	// settings 主体:Content = 原文件文本(UI 展示用);Fields 保留 model/env 及全文本载体 raw。
	//
	// 展示契约(修 ContentArea structured 泄漏):
	//   - Content = string(data):UI 一律从 content 取展示文本,不再从 fields 拼凑。
	//     旧实现只把文件字节塞进 Fields["raw],API 序列化后前端拿到 {model:"",env,raw:{整文件}}
	//     包装结构,文件被冗余包在 raw 里、空 model 误导用户。现 content 直出原文件。
	//   - raw 仍是全文本载体,供规则引擎 baseline 规则 field: raw 全文本匹配
	//     (skipDangerousModePermissionPrompt / curl 远程脚本等)。类型从 json.RawMessage 改 string:
	//     stringify 两者兼容(eval.go),但 string 经 API marshal 为 JSON 字符串,前端 typeof==='string'
	//     成立,消除「raw 是对象还是字符串」的歧义。
	//   - model:文件有 model 键才放进 Fields(省略空值,避免 "" 被误读为「模型配置为空」)。
	base := Asset{Type: AssetSettings, Scope: scope, SourcePath: path, Name: baseName, Content: string(data)}
	base.Fields = map[string]any{
		"env": rs.Env,
		"raw": string(data),
	}
	if rs.Model != "" {
		base.Fields["model"] = rs.Model
	}
	fillHash(&base)
	out = append(out, base)

	// permissions 单列,便于基线检测器按类型匹配。
	// Content = 父文件文本(settings.json):permissions 是 settings.json 的切片,
	// 点开看父文件全文(含上下文)比看 allow/deny/ask 孤立清单更有用。
	perm := Asset{Type: AssetPermissions, Scope: scope, SourcePath: path, Name: "permissions", Content: string(data)}
	perm.Fields = map[string]any{
		"allow": rs.Permissions.Allow,
		"deny":  rs.Permissions.Deny,
		"ask":   rs.Permissions.Ask,
	}
	fillHash(&perm)
	out = append(out, perm)

	// 每个 hook 一条资产(逻辑抽到 parseHooksFromData,供插件 hook 复用)。
	// Name 带上 entry 索引(ei)与 hook 索引(hi):
	// - 同一 matcher 下可挂多个 hook(within-entry,hi 区分);
	// - 同一 event 下多条 entry 的 matcher 字符串可能相同(cross-entry,ei 区分)。
	// 否则 scope:type:name:source_path 相同 → ID 重复,下游去重/聚合会悄悄丢弃重复 hook。
	// event/matcher 仍保留在 Fields 里供查询。slice 顺序在 json.Unmarshal 下确定,
	// 故 ID 跨运行可复现。
	out = append(out, parseHooksFromData(data, path, scope)...)
	return out, nil
}

// settingsBaseName 按文件名推导 settings 资产 base 名:
// settings.json → "settings";settings.local.json → "settings.local";其余按去扩展名处理。
// 使 settings + settings.local 同存时 Name 不同 → ID 不同(防去重丢弃)+ 树独立节点。
func settingsBaseName(path string) string {
	base := filepath.Base(path)
	switch base {
	case "settings.json":
		return "settings"
	case "settings.local.json":
		return "settings.local"
	}
	// 兜底:其他同名文件按去 .json 扩展名处理。
	if ext := filepath.Ext(base); ext != "" {
		return strings.TrimSuffix(base, ext)
	}
	return base
}

// fillHash 填充 Hash/MTime(来自源文件)与 ID。
// 文件不可读时 Hash/MTime 留空但仍设 ID;由所有解析路径(含损坏文件分支)调用。
func fillHash(a *Asset) {
	if h, mt, err := HashAndMTime(a.SourcePath); err == nil {
		a.Hash, a.MTime = h, mt
	}
	a.ID = makeAssetID(*a)
}
