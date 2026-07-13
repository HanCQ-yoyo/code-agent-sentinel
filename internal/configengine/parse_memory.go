package configengine

import (
	"os"
	"path/filepath"
	"strings"
)

// parseMemory 解析 CLAUDE.md + memory/ 目录(每条 memory 文件一条资产)。
//
// CLAUDE.md 的 Content 为原始文件内容(含 frontmatter,不拆分)。
// memory/ 下每个 .md 文件产出一条资产。两者均无时不致错。
func parseMemory(claudeDir string, scope Scope) ([]Asset, error) {
	var out []Asset
	// CLAUDE.md 与 CLAUDE.local.md:项目根/全局的标准记忆文件。归入 memory 类型,
	// Name 区分为文件名(供 UI 与去重区分)。
	for _, name := range []string{"CLAUDE.md", "CLAUDE.local.md"} {
		p := filepath.Join(claudeDir, name)
		if !fileExists(p) {
			continue
		}
		data, _ := os.ReadFile(p)
		a := Asset{Type: AssetMemory, Scope: scope, SourcePath: p, Name: name, Content: string(data)}
		fillHash(&a)
		out = append(out, a)
	}
	memDir := filepath.Join(claudeDir, "memory")
	entries, err := os.ReadDir(memDir)
	if err != nil {
		return out, nil
	}
	for _, en := range entries {
		if en.IsDir() || !strings.HasSuffix(en.Name(), ".md") {
			continue
		}
		p := filepath.Join(memDir, en.Name())
		data, _ := os.ReadFile(p)
		a := Asset{Type: AssetMemory, Scope: scope, SourcePath: p, Name: en.Name(), Content: string(data)}
		fillHash(&a)
		out = append(out, a)
	}
	return out, nil
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }
