package api

import (
	"encoding/json"
)

// jsonMarshalDirTags 序列化 DirTags 为 JSON 字符串。
func jsonMarshalDirTags(m map[string]string) string {
	b, _ := json.Marshal(m)
	return string(b)
}

// jsonUnmarshalDirTags 从 JSON 字符串反序列化到 DirTags。
func jsonUnmarshalDirTags(s string, v *map[string]string) {
	_ = json.Unmarshal([]byte(s), v)
}
