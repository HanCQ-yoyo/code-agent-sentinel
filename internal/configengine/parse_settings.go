package configengine

import (
	"encoding/json"
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
		// 注意:Asset 无 ID() 方法(字段与方法同名冲突),用 makeAssetID 设置。
		a := Asset{Type: AssetSettings, Scope: scope, SourcePath: path, Name: "settings", ParseError: err.Error()}
		a.ID = makeAssetID(a)
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

	// 每个 hook 一条资产,event/matcher 唯一标识其用途。
	for event, entries := range rs.Hooks {
		for _, e := range entries {
			for _, h := range e.Hooks {
				hk := Asset{Type: AssetHook, Scope: scope, SourcePath: path, Name: event + "/" + e.Matcher}
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
// 对解析失败的资产不调用(见 parseSettings 损坏分支),那里只设 ID。
func fillHash(a *Asset) {
	if h, mt, err := HashAndMTime(a.SourcePath); err == nil {
		a.Hash, a.MTime = h, mt
	}
	a.ID = makeAssetID(*a)
}
