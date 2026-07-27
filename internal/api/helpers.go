package api

import (
	"strings"
	"time"
)

// nowUTC 返回当前 UTC 时间的 RFC3339 字符串(处置状态时间戳用)。
// 项目已有内联先例(handlers_suppressions.go / handlers_scheduler.go),抽 helper 便于复用。
func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// splitCSV 按逗号切分字符串,去前后空白与空段(用于 ?active=f1,f2,f3 查询参数)。
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
