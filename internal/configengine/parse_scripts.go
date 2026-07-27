package configengine

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// scriptExt 判断扩展名是否为脚本(.sh/.py/.js/.ts/.bash),与 scriptArgRe 集合一致。
// parseHooksScriptDir 用它过滤 hooks/ 目录条目(规格 §2.1/§8.1 独立 hook 脚本目录)。
func scriptExt(ext string) bool {
	switch ext {
	case ".sh", ".py", ".js", ".ts", ".bash":
		return true
	}
	return false
}

// parseHooksScriptDir 枚举 hooks/ 目录下的脚本文件(.sh/.py/.js/.ts/.bash 等),
// 每个产出一条 script 资产(规格 §2.1/§8.1 独立 hook 脚本目录)。子目录不递归。
//
// 与 parseScripts(从 hook/command 的 command 字段抽取引用脚本)互补:后者只发现被引用的
// 脚本,而 hooks/ 目录下的脚本可能被外部(如 CI/cron)直接调用,parseHooksScriptDir 确保
// 它们也被纳入安全扫描。两者可能对同一个 hooks/ 脚本各产出一条 script 资产(hooks/ 枚举
// 一条 + hook command 引用一条)——由 parseScripts 在调用时预填 seen(传入 assets 中已有
// AssetScript 的 SourcePath)去重,parseHooksScriptDir 先于 parseScripts 运行,故其产出
// 的 script 资产 SourcePath 会被 parseScripts 跳过,不会重复入表。
// detectDuplicates 另行按 type+name 上报(不删除)跨 scope / 跨 source_path 的重复,供 UI 展示。
//
// Content 读取文件文本(与 parseScripts 一致),供 secret/dependency 检测器扫描。
func parseHooksScriptDir(claudeDir string, scope Scope) []Asset {
	hooksDir := filepath.Join(claudeDir, "hooks")
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		return nil // hooks/ 不存在或不可读,静默跳过(不算错误)
	}
	var out []Asset
	for _, en := range entries {
		if en.IsDir() {
			continue // 子目录不递归(规格只要求枚举顶层脚本)
		}
		ext := strings.ToLower(filepath.Ext(en.Name()))
		if !scriptExt(ext) {
			continue // 非脚本扩展名跳过(README/配置等)
		}
		p := filepath.Join(hooksDir, en.Name())
		data, _ := os.ReadFile(p) // 读不到 Content 留空,不致失败
		a := Asset{Type: AssetScript, Scope: scope, SourcePath: p, Name: en.Name(), Content: string(data)}
		fillHash(&a)
		out = append(out, a)
	}
	return out
}

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
//
//	全局:claudeDir = ~/.claude → base = ~ (home,Claude Code 跑 hook 的目录);
//	项目:claudeDir = <project>/.claude → base = <project>。
//
// 解析后不存在的脚本仍跳过(宁跳不发错路径)。claudeDir 现在真正被使用。
func parseScripts(assets []Asset, claudeDir string) []Asset {
	base := filepath.Dir(claudeDir)
	seen := map[string]bool{}
	// 预填 seen:把调用方传入的 assets 中已有的 AssetScript 的 SourcePath 登记下来,
	// 使 parseHooksScriptDir 先于 parseScripts 产出的 hooks/ 脚本资产不会被这里再次抽取
	// (hook command 常以绝对路径引用 ~/.claude/hooks/pre.sh,与 parseHooksScriptDir 的
	// SourcePath 一致 → 命中 seen,跳过)。两者路径都已是绝对路径(parseHooksScriptDir 用
	// filepath.Join(hooksDir, name);parseScripts 对绝对路径不再改写),无需额外归一化。
	// 对相对路径脚本(parseScripts 自身会 base-解析)不在预填范围——这是 per-call 内部去重,
	// 不影响 parseScripts 自身对相对路径的处理。
	for _, a := range assets {
		if a.Type != AssetScript || a.SourcePath == "" {
			continue
		}
		// SourcePath 已是绝对路径(parseHooksScriptDir 保证);Clean 一次确保与下面
		// parseScripts 对绝对路径分支(原样使用)的对比一致。
		seen[filepath.Clean(a.SourcePath)] = true
	}
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
