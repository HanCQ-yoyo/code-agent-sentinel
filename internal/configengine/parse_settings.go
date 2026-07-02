package configengine

import (
	"encoding/json"
	"fmt"
	"os"
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

// parseSettings 解析 settings.json,产出 settings + permissions + 每个 hook 一条资产。
//
// 损坏文件不致失败:返回一条带 parse_error 的 settings 占位资产,供上层作为 Finding 暴露。
func parseSettings(path string, scope Scope) ([]Asset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rs rawSettings
	if err := json.Unmarshal(data, &rs); err != nil {
		// 损坏文件:产出一条带 parse_error 的 settings 占位资产。
		// 文件本身可读,故仍填 hash/mtime(与 placeholder 行为一致);fillHash 内部会设 ID。
		a := Asset{Type: AssetSettings, Scope: scope, SourcePath: path, Name: "settings", ParseError: err.Error()}
		fillHash(&a)
		return []Asset{a}, nil
	}
	var out []Asset

	// settings 主体:保留 model/env 及原始 JSON。
	base := Asset{Type: AssetSettings, Scope: scope, SourcePath: path, Name: "settings"}
	base.Fields = map[string]any{
		"model": rs.Model,
		"env":   rs.Env,
		"raw":   json.RawMessage(data),
	}
	fillHash(&base)
	out = append(out, base)

	// permissions 单列,便于基线检测器按类型匹配。
	perm := Asset{Type: AssetPermissions, Scope: scope, SourcePath: path, Name: "permissions"}
	perm.Fields = map[string]any{
		"allow": rs.Permissions.Allow,
		"deny":  rs.Permissions.Deny,
		"ask":   rs.Permissions.Ask,
	}
	fillHash(&perm)
	out = append(out, perm)

	// 每个 hook 一条资产。Name 带内层索引:同一 matcher 下可挂多个 hook,
	// 若只用 event/matcher 会导致 scope:type:name:source_path 相同 → ID 重复,
	// 下游去重/聚合会悄悄丢弃重复 hook。event/matcher 仍保留在 Fields 里供查询。
	for event, entries := range rs.Hooks {
		for _, e := range entries {
			for idx, h := range e.Hooks {
				hk := Asset{Type: AssetHook, Scope: scope, SourcePath: path, Name: fmt.Sprintf("%s/%s/%d", event, e.Matcher, idx)}
				hk.Fields = map[string]any{
					"event":   event,
					"matcher": e.Matcher,
					"type":    h.Type,
					"command": h.Command,
				}
				fillHash(&hk)
				out = append(out, hk)
			}
		}
	}
	return out, nil
}

// fillHash 填充 Hash/MTime(来自源文件)与 ID。
// 文件不可读时 Hash/MTime 留空但仍设 ID;由所有解析路径(含损坏文件分支)调用。
func fillHash(a *Asset) {
	if h, mt, err := HashAndMTime(a.SourcePath); err == nil {
		a.Hash, a.MTime = h, mt
	}
	a.ID = makeAssetID(*a)
}
