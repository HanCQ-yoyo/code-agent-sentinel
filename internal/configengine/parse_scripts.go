package configengine

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// scriptArgRe 匹配命令行中以 .sh/.py/.js/.ts/.bash 结尾的脚本路径参数。
// (?:^|\s) 与 (?:\s|$) 允许前缀(如 "bash ")和行尾;路径不含空白/引号。
var scriptArgRe = regexp.MustCompile(`(?:^|\s)([^\s'"]+\.(?:sh|py|js|ts|bash))(?:\s|$)`)

// parseScripts 从 hook/command 资产的 command 字段抽取引用的脚本路径,
// 存在则产出 script 资产(含文件内容)。claudeDir 当前未使用,保留以便
// 后续增强(如解析相对路径、plugins 内脚本)。
func parseScripts(assets []Asset, claudeDir string) []Asset {
	seen := map[string]bool{}
	var out []Asset
	for _, a := range assets {
		if a.Type != AssetHook && a.Type != AssetCommand {
			continue
		}
		cmd, _ := a.Fields["command"].(string)
		if cmd == "" {
			continue
		}
		for _, m := range scriptArgRe.FindAllString(cmd, -1) {
			p := strings.TrimSpace(m)
			if !fileExists(p) || seen[p] {
				continue
			}
			seen[p] = true
			data, _ := os.ReadFile(p)
			// 用 filepath.Base 而非自造 baseName:stdlib 已覆盖跨平台分隔符,DRY。
			s := Asset{Type: AssetScript, Scope: a.Scope, SourcePath: p, Name: filepath.Base(p), Content: string(data)}
			fillHash(&s)
			out = append(out, s)
		}
	}
	return out
}
