package configengine

// Duplicate 表示同类型同名的资产出现在多个 scope 或多个 source_path,通常意味着
// 配置漂移或覆盖关系(如全局与项目都定义了同名 MCP server)。
type Duplicate struct {
	Type     AssetType `json:"type"`
	Name     string    `json:"name"`
	AssetIDs []string  `json:"asset_ids"`
}

// detectDuplicates 找出同类型同名的资产(跨 scope / 跨 source_path)。
//
// 分组键为 type:name;同组内出现 ≥2 条即报为 Duplicate,AssetIDs 列出全部成员 ID。
// 注意:调用方需保证传入的 assets 不含重复 ID(同一资产被误入两次),否则会误报。
// Discover 在 discoverProject 中通过限定 parseScripts 的输入 scope 来避免此情况。
func detectDuplicates(assets []Asset) []Duplicate {
	key := map[string][]Asset{}
	for _, a := range assets {
		k := string(a.Type) + ":" + a.Name
		key[k] = append(key[k], a)
	}
	var out []Duplicate
	for _, group := range key {
		if len(group) < 2 {
			continue
		}
		ids := make([]string, len(group))
		for i, a := range group {
			ids[i] = a.ID
		}
		out = append(out, Duplicate{Type: group[0].Type, Name: group[0].Name, AssetIDs: ids})
	}
	return out
}
