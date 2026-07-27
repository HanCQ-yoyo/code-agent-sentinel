package configengine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCredentialsAuthJson(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "auth.json"), []byte(`{"token":"sk-xxx"}`), 0o644)
	assets := parseCredentials(dir, ScopeGlobal)
	if len(assets) != 1 {
		t.Fatalf("got %d credential assets, want 1", len(assets))
	}
	a := assets[0]
	if a.Type != AssetCredential {
		t.Fatalf("type = %v, want credential", a.Type)
	}
	if a.Fields["kind"] != "auth" {
		t.Fatalf("kind = %v, want auth", a.Fields["kind"])
	}
	if a.Content != "" {
		t.Fatalf("credential Content 必须为空(不暴露内容), got %q", a.Content)
	}
	if a.Hash == "" {
		t.Fatal("credential 必须有 hash(存在性指纹)")
	}
}

func TestParseCredentialsEnvAndKey(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte("FOO=bar"), 0o644)
	os.WriteFile(filepath.Join(dir, "id_rsa.pem"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, ".netrc"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("x"), 0o644) // 非凭据,跳过
	assets := parseCredentials(dir, ScopeProject)
	kinds := map[string]bool{}
	for _, a := range assets {
		kinds[a.Fields["kind"].(string)] = true
	}
	if !kinds["env"] || !kinds["key"] || !kinds["netrc"] {
		t.Fatalf("缺凭据类型, got kinds=%v", kinds)
	}
	if len(assets) != 3 {
		t.Fatalf("got %d assets, want 3(不含 readme.md)", len(assets))
	}
}

func TestCredentialKindMapping(t *testing.T) {
	cases := map[string]string{
		"auth.json":        "auth",
		"AUTH.JSON":        "auth", // 大写也命中(ToLower)
		".env":             "env",
		".env.local":       "env",
		".ENV":             "env", // 大写也命中(ToLower,修前漏)
		".Env.Local":       "env", // 混合大小写也命中
		"server.pem":       "key",
		"private.key":      "key",
		".netrc":           "netrc",
		".git-credentials": "netrc",
		"random.txt":       "",
	}
	for name, want := range cases {
		if got := credentialKind(name); got != want {
			t.Errorf("credentialKind(%q) = %q, want %q", name, got, want)
		}
	}
}
