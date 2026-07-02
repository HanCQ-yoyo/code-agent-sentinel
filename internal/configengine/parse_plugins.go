package configengine

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// parsePlugins 遍历 plugins/cache/<marketplace>/<plugin>/,每个产出一条 plugin 资产。
// cache 目录或子目录不存在时不致错,返回 nil。package.json 解析失败时 Name/Version
// 留空但资产仍产出(存在性即有价值)。
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
			pj := filepath.Join(cache, m.Name(), p.Name(), "package.json")
			if !fileExists(pj) {
				continue
			}
			var meta struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			}
			data, _ := os.ReadFile(pj)
			_ = json.Unmarshal(data, &meta)
			a := Asset{Type: AssetPlugin, Scope: scope, SourcePath: filepath.Join(cache, m.Name(), p.Name()), Name: p.Name()}
			a.Fields = map[string]any{"name": meta.Name, "version": meta.Version, "marketplace": m.Name()}
			fillHash(&a)
			out = append(out, a)
		}
	}
	return out, nil
}
