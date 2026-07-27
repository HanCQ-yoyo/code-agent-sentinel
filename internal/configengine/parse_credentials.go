package configengine

import (
	"os"
	"path/filepath"
	"strings"
)

// credentialKind 按文件名推断凭据类型。非凭据文件返回空字符串。
// 映射(规格 §3.5/§8.3):
//   - auth.json → auth
//   - .env / .env.* → env
//   - *.pem / *.key → key
//   - .netrc / .git-credentials → netrc
func credentialKind(name string) string {
	lower := strings.ToLower(name)
	switch lower {
	case "auth.json":
		return "auth"
	case ".netrc", ".git-credentials":
		return "netrc"
	}
	if lower == ".env" || strings.HasPrefix(lower, ".env.") {
		return "env"
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".pem" || ext == ".key" {
		return "key"
	}
	return ""
}

// parseCredentials 扫描 dir 顶层的凭据文件,每个产出一条 credential 资产。
// Content 留空(不暴露凭据明文);Fields 只存 path/kind;hash 用文件内容算存在性指纹。
// 非凭据文件跳过;目录不递归(只扫顶层,避免扫进 .git 等)。
func parseCredentials(dir string, scope Scope) []Asset {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Asset
	for _, en := range entries {
		if en.IsDir() {
			continue
		}
		kind := credentialKind(en.Name())
		if kind == "" {
			continue
		}
		p := filepath.Join(dir, en.Name())
		a := Asset{Type: AssetCredential, Scope: scope, SourcePath: p, Name: en.Name()}
		a.Fields = map[string]any{"path": p, "kind": kind}
		fillHash(&a) // 内容 hash(不暴露,仅存指纹供变更检测)
		out = append(out, a)
	}
	return out
}
