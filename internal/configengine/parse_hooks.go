package configengine

import (
	"encoding/json"
	"fmt"
)

// parseHooksFromData 解析 settings 风格的 hooks map,每个 hook 产出一条 AssetHook。
//
// 抽自 parseSettings,供插件 hooks/hooks.json 复用。接受两种布局:
//   - settings.json 整体:hooks 嵌套在 "hooks" 键下(如 {"model": ..., "hooks": {...}})。
//   - hooks.json 整体:顶层即为 hooks map(如 {"PreToolUse": [...]}),插件 hook 用此布局。
//
// 损坏 JSON 返回 nil(不产出占位);调用方对损坏文件自行决定是否发 parse_error 资产。
// Name 格式 event/matcher/ei/hi 与 parseSettings 一致,保证 ID 跨来源可复现。
func parseHooksFromData(data []byte, path string, scope Scope) []Asset {
	// 先按 settings.json 布局尝试(hooks 嵌套在 "hooks" 键下)。
	var rs struct {
		Hooks map[string][]rawHookEntry `json:"hooks"`
	}
	if err := json.Unmarshal(data, &rs); err == nil && rs.Hooks != nil {
		return hooksToAssets(rs.Hooks, data, path, scope)
	}
	// 再按 hooks.json 布局尝试(顶层即为 hooks map)。
	var hooks map[string][]rawHookEntry
	if err := json.Unmarshal(data, &hooks); err != nil {
		return nil
	}
	return hooksToAssets(hooks, data, path, scope)
}

// hooksToAssets 把已解析的 hooks map 转成 AssetHook 列表。
// Name 带上 entry 索引(ei)与 hook 索引(hi)以避免 ID 重复:
//   - 同一 matcher 下可挂多个 hook(within-entry,hi 区分);
//   - 同一 event 下多条 entry 的 matcher 字符串可能相同(cross-entry,ei 区分)。
//
// slice 顺序在 json.Unmarshal 下确定,故 ID 跨运行可复现。
//
// Content = 父文件文本(data):hook 是 settings.json/hooks.json 的切片,点开看父文件
// 全文(含 event 上下文)比看 command 孤立串更有用。data 由调用方传入(parseSettings
// 传 settings.json 字节,插件 hooks.json 传其字节),故 Content 对应各自来源文件。
func hooksToAssets(hooks map[string][]rawHookEntry, data []byte, path string, scope Scope) []Asset {
	var out []Asset
	for event, entries := range hooks {
		for ei, e := range entries {
			for hi, h := range e.Hooks {
				hk := Asset{Type: AssetHook, Scope: scope, SourcePath: path, Name: fmt.Sprintf("%s/%s/%d/%d", event, e.Matcher, ei, hi), Content: string(data)}
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
	return out
}
