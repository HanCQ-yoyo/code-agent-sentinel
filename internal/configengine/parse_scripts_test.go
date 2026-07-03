package configengine

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseScriptsResolvesRelativePath 验证 I-CORR-5:hook/command 的 command
// 字段常以相对路径引用脚本(如 "bash scripts/foo.sh")。旧实现把原始相对路径
// 存进 SourcePath,fileExists/os.ReadFile 解析到 sentinel 进程 CWD(错误),
// secret/dep 检测器再 filepath.Dir(相对路径) 扫错目录。修复:相对路径以
// claudeDir 的父目录(home 或 project 根)为 base 解析为绝对路径。
//
// 不触真实 ~/.claude:用 newFixture 临时目录 + 手动构造 hook 资产 + 直接调
// parseScripts(绕过 Discover,聚焦被测函数)。
func TestParseScriptsResolvesRelativePath(t *testing.T) {
	f := newFixture(t)
	// base = filepath.Dir(claudeDir) = home。脚本放 home/scripts/foo.sh。
	scriptRel := filepath.Join("scripts", "foo.sh")
	if err := os.MkdirAll(filepath.Join(f.home, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	absScript := filepath.Join(f.home, scriptRel)
	if err := os.WriteFile(absScript, []byte("#!/bin/bash\necho hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// hook 资产的 command 用相对路径引用脚本(模拟 settings.json 里的真实写法)。
	hook := Asset{
		Type:       AssetHook,
		Scope:      ScopeGlobal,
		SourcePath: filepath.Join(f.claude, "settings.json"),
		Name:       "PreToolUse/*",
		Fields:     map[string]any{"command": "bash " + scriptRel},
	}

	out := parseScripts([]Asset{hook}, f.claude)
	if len(out) != 1 {
		t.Fatalf("期望 1 个 script 资产,实际 %d: %+v", len(out), out)
	}
	s := out[0]
	if !filepath.IsAbs(s.SourcePath) {
		t.Errorf("SourcePath 应为绝对路径,实际 %q", s.SourcePath)
	}
	// 应指向真实文件(base = home)。
	want, _ := filepath.Abs(absScript)
	if filepath.Clean(s.SourcePath) != filepath.Clean(want) {
		t.Errorf("SourcePath = %q, want %q", s.SourcePath, want)
	}
	if s.Content != "#!/bin/bash\necho hi\n" {
		t.Errorf("Content 未正确读取: %q", s.Content)
	}
}

// TestParseScriptsAbsolutePathUnchanged 验证绝对路径脚本不被改写(base 仅用于相对路径)。
func TestParseScriptsAbsolutePathUnchanged(t *testing.T) {
	f := newFixture(t)
	absScript := filepath.Join(f.home, "deploy.sh")
	os.WriteFile(absScript, []byte("#!/bin/sh\nexit 0\n"), 0o644)

	hook := Asset{
		Type:   AssetHook,
		Scope:  ScopeGlobal,
		Name:   "PostToolUse",
		Fields: map[string]any{"command": "sh " + absScript},
	}
	out := parseScripts([]Asset{hook}, f.claude)
	if len(out) != 1 {
		t.Fatalf("期望 1 个 script 资产,实际 %d", len(out))
	}
	if filepath.Clean(out[0].SourcePath) != filepath.Clean(absScript) {
		t.Errorf("绝对路径被改写: got %q, want %q", out[0].SourcePath, absScript)
	}
}

// TestParseScriptsRelativeMissingSkipped 验证相对路径解析后不存在则跳过
// (current 行为保留:宁跳不发错路径)。
func TestParseScriptsRelativeMissingSkipped(t *testing.T) {
	f := newFixture(t)
	hook := Asset{
		Type:   AssetHook,
		Scope:  ScopeGlobal,
		Name:   "PreToolUse",
		Fields: map[string]any{"command": "bash scripts/does-not-exist.sh"},
	}
	out := parseScripts([]Asset{hook}, f.claude)
	if len(out) != 0 {
		t.Errorf("不存在的相对脚本应被跳过,实际 %d: %+v", len(out), out)
	}
}
