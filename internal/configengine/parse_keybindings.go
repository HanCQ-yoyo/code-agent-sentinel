package configengine

import (
	"encoding/json"
	"os"
)

// parseKeybindings 解析 keybindings.json(键→动作字符串映射)。
// 文件不存在返回 nil,nil;损坏 JSON 产出一条带 parse_error 的占位资产(文件可读,
// 故仍填 hash/mtime)。正常时每个键值对写入 Fields。
func parseKeybindings(path string, scope Scope) ([]Asset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	var kb map[string]string
	if err := json.Unmarshal(data, &kb); err != nil {
		// 损坏文件:文件本身可读,fillHash 内部会设 Hash/MTime/ID。
		// (brief 原文为 a.ID = a.ID(),但 Asset 无 ID() 方法且字段与方法不能同名,
		// 故用 fillHash,与 parseSettings/parseMemory 的损坏分支一致。)
		a := Asset{Type: AssetKeybinding, Scope: scope, SourcePath: path, Name: "keybindings", ParseError: err.Error()}
		fillHash(&a)
		return []Asset{a}, nil
	}
	a := Asset{Type: AssetKeybinding, Scope: scope, SourcePath: path, Name: "keybindings", Fields: map[string]any{}}
	for k, v := range kb {
		a.Fields[k] = v
	}
	fillHash(&a)
	return []Asset{a}, nil
}
