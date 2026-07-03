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
// 存在则产出 script 资产(含文件内容)。
//
// I-CORR-5:command 常以相对路径引用脚本(如 "bash scripts/deploy.sh")。
// 旧实现直接用相对 p,fileExists/os.ReadFile 解析到 sentinel 进程 CWD(错误),
// 且 SourcePath 留相对路径 → secret/dep 检测器 filepath.Dir(相对) 扫错目录,
// 生产环境静默失效。修复:相对路径以 claudeDir 的父目录为 base 解析为绝对——
//   全局:claudeDir = ~/.claude → base = ~ (home,Claude Code 跑 hook 的目录);
//   项目:claudeDir = <project>/.claude → base = <project>。
// 解析后不存在的脚本仍跳过(宁跳不发错路径)。claudeDir 现在真正被使用。
func parseScripts(assets []Asset, claudeDir string) []Asset {
	base := filepath.Dir(claudeDir)
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
			if !filepath.IsAbs(p) {
				p = filepath.Join(base, p)
				p = filepath.Clean(p)
			}
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
