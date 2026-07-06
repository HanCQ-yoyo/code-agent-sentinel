package configengine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// pluginManifest 是 .claude-plugin/plugin.json 的字段子集。
type pluginManifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"author"`
}

// parsePlugins 遍历 plugins/cache/<marketplace>/<plugin>/<version>/,
// 每个插件产出一条 plugin 资产 + 下钻其版本目录内的 skills/commands/agents/hooks
// (scope=plugin)。多版本取最高(简单 sort)。cache 不存在返回 nil。
//
// 修复:旧实现找 <plugin>/package.json,但真实布局多一层版本目录、清单是
// .claude-plugin/plugin.json —— 旧实现产 0 插件。下钻后 skills/commands/agents
// 成为 plugin-scope 资产,InjectionDetector.Covers 已含 → 自动被提示注入扫描。
func parsePlugins(claudeDir string, scope Scope) ([]Asset, error) {
	cache := filepath.Join(claudeDir, "plugins", "cache")
	mkts, err := os.ReadDir(cache)
	if err != nil {
		return nil, nil
	}
	var out []Asset
	for _, m := range mkts {
		if !m.IsDir() {
			continue
		}
		plugs, err := os.ReadDir(filepath.Join(cache, m.Name()))
		if err != nil {
			continue
		}
		for _, p := range plugs {
			if !p.IsDir() {
				continue
			}
			// 版本目录:<plugin>/<version>/
			versions, err := os.ReadDir(filepath.Join(cache, m.Name(), p.Name()))
			if err != nil {
				continue
			}
			versionDir, manifest := pickHighestVersion(cache, m.Name(), p.Name(), versions)
			if versionDir == "" {
				continue // 无可用清单的插件跳过
			}
			a := Asset{Type: AssetPlugin, Scope: scope, SourcePath: versionDir, Name: p.Name()}
			a.Fields = map[string]any{
				"name":        manifest.Name,
				"version":     manifest.Version,
				"description": manifest.Description,
				"author":      manifest.Author.Name,
				"marketplace": m.Name(),
			}
			fillHash(&a)
			out = append(out, a)

			// 下钻:skills/commands/agents markdown 目录(plugin scope,标注 plugin)。
			for _, sub := range []struct {
				rel string
				typ AssetType
			}{
				{"skills", AssetSkill},
				{"commands", AssetCommand},
				{"agents", AssetAgent},
			} {
				if assets, _ := parseMarkdownDir(filepath.Join(versionDir, sub.rel), sub.typ, ScopePlugin); assets != nil {
					for i := range assets {
						if assets[i].Fields == nil {
							assets[i].Fields = map[string]any{}
						}
						assets[i].Fields["plugin"] = p.Name()
						assets[i].Fields["marketplace"] = m.Name()
					}
					out = append(out, assets...)
				}
			}

			// 下钻:hooks/hooks.json(plugin scope)。
			if hp := filepath.Join(versionDir, "hooks", "hooks.json"); fileExists(hp) {
				if data, err := os.ReadFile(hp); err == nil {
					hooks := parseHooksFromData(data, hp, ScopePlugin)
					for i := range hooks {
						if hooks[i].Fields == nil {
							hooks[i].Fields = map[string]any{}
						}
						hooks[i].Fields["plugin"] = p.Name()
						hooks[i].Fields["marketplace"] = m.Name()
					}
					out = append(out, hooks...)
				}
			}
		}
	}
	return out, nil
}

// pickHighestVersion 在 <plugin>/ 下找版本目录,返回最高版本的绝对路径 + 解析出的清单。
// 版本目录判定:含 .claude-plugin/plugin.json。无清单的版本目录跳过。无任何可用版本
// 返回 ("", pluginManifest{})。
func pickHighestVersion(cache, market, plugin string, versions []os.DirEntry) (string, pluginManifest) {
	type cand struct {
		ver      string
		dir      string
		manifest pluginManifest
	}
	var cands []cand
	for _, v := range versions {
		if !v.IsDir() {
			continue
		}
		mp := filepath.Join(cache, market, plugin, v.Name(), ".claude-plugin", "plugin.json")
		if !fileExists(mp) {
			continue
		}
		data, err := os.ReadFile(mp)
		if err != nil {
			continue
		}
		var man pluginManifest
		_ = json.Unmarshal(data, &man) // 解析失败留空字段,不阻断
		cands = append(cands, cand{ver: v.Name(), dir: filepath.Join(cache, market, plugin, v.Name()), manifest: man})
	}
	if len(cands) == 0 {
		return "", pluginManifest{}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].ver > cands[j].ver })
	return cands[0].dir, cands[0].manifest
}
