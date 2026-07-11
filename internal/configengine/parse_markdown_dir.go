package configengine

import (
	"os"
	"path/filepath"
	"strings"
)

// parseMarkdownDir 遍历目录,每个含 markdown 的顶层条目产出一条资产。
//
// 顶层条目可以是子目录(在其中找 .md)或单个 .md 文件。每个条目产出一条资产,
// Content 为正文(去掉 frontmatter),Fields 含解析出的 frontmatter
// name/description/allowed-tools。目录不存在时返回 nil, nil(不致错)。
func parseMarkdownDir(dir string, typ AssetType, scope Scope) ([]Asset, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	var out []Asset
	for _, en := range entries {
		name, mdPath := entryMarkdown(dir, en)
		if mdPath == "" {
			continue
		}
		data, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}
		a := Asset{Type: typ, Scope: scope, SourcePath: mdPath, Name: name}
		fm, body := splitFrontmatter(string(data))
		a.Fields = map[string]any{
			"name":          fm["name"],
			"description":   fm["description"],
			"allowed-tools": fm["allowed-tools"],
		}
		a.Content = body
		fillHash(&a)
		out = append(out, a)
	}
	return out, nil
}

// entryMarkdown 解析一个目录条目:若是目录,找其中的 *.md;若是 .md 文件,直接用。
//
// 子目录里有多个 .md 时只取第一个(Claude Code 约定每个目录一个主文件
// SKILL.md/COMMAND.md);多 md 目录不会被完整枚举。
func entryMarkdown(dir string, en os.DirEntry) (name, path string) {
	if en.IsDir() {
		sub := filepath.Join(dir, en.Name())
		entries, err := os.ReadDir(sub)
		if err != nil {
			return "", ""
		}
		for _, c := range entries {
			if !c.IsDir() && strings.HasSuffix(c.Name(), ".md") {
				return en.Name(), filepath.Join(sub, c.Name())
			}
		}
		return "", ""
	}
	if strings.HasSuffix(en.Name(), ".md") {
		return strings.TrimSuffix(en.Name(), ".md"), filepath.Join(dir, en.Name())
	}
	return "", ""
}

// splitFrontmatter 分离 YAML frontmatter(--- 分隔)与正文。
// 无 frontmatter 或分隔不完整时返回空 map 与原文。
func splitFrontmatter(s string) (map[string]string, string) {
	fm := map[string]string{}
	if !strings.HasPrefix(s, "---\n") {
		return fm, s
	}
	rest := strings.TrimPrefix(s, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return fm, s
	}
	head := rest[:idx]
	body := rest[idx+len("\n---\n"):]
	for _, line := range strings.Split(head, "\n") {
		if i := strings.Index(line, ":"); i > 0 {
			k := strings.TrimSpace(line[:i])
			v := strings.Trim(strings.TrimSpace(line[i+1:]), "\"")
			fm[k] = v
		}
	}
	return fm, body
}
