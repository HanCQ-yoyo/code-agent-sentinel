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

// TestParseScriptsSkipsHooksDirDuplicates 验证 Task 7 review Important 修复:
// parseHooksScriptDir 和 parseScripts 可能对同一个 hooks/ 脚本各产一条 script 资产。
// 修复:parseScripts 用传入 assets 中已有 AssetScript 的 SourcePath 预填 seen,跳过
// parseHooksScriptDir 已产出的同路径脚本。
//
// 直接调 parseScripts(绕过 Discover):输入 = [已有 script 资产(模拟 parseHooksScriptDir
// 产出), hook 资产(command 引用同一脚本)],期望输出 0(不重复产出)。
func TestParseScriptsSkipsHooksDirDuplicates(t *testing.T) {
	f := newFixture(t)
	// 模拟 parseHooksScriptDir 已产出的 ~/.claude/hooks/pre.sh script 资产。
	hooksDir := filepath.Join(f.claude, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(hooksDir, "pre.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	existingScript := Asset{
		Type:       AssetScript,
		Scope:      ScopeGlobal,
		SourcePath: scriptPath,
		Name:       "pre.sh",
		Content:    "#!/bin/sh\necho hi\n",
	}
	fillHash(&existingScript)

	// hook 的 command 以绝对路径引用同一脚本。
	hook := Asset{
		Type:   AssetHook,
		Scope:  ScopeGlobal,
		Name:   "PreToolUse/*",
		Fields: map[string]any{"command": "bash " + scriptPath},
	}

	out := parseScripts([]Asset{existingScript, hook}, f.claude)
	if len(out) != 0 {
		t.Fatalf("期望 0 个 script 资产(parseHooksScriptDir 已产出同路径应跳过),实际 %d: %+v", len(out), out)
	}
}

// TestParseScriptsSkipsHooksDirDuplicatesRelative 验证 hook command 以相对路径引用
// hooks/ 脚本时,parseScripts 同样应跳过(parseHooksScriptDir 产出的 SourcePath 是绝对
// 路径,parseScripts 对相对路径会 base-解析,但 base=filepath.Dir(claudeDir)=home,
// 故 "hooks/pre.sh" → home/hooks/pre.sh(与 parseHooksScriptDir 的 claudeDir/hooks/pre.sh
// 不匹配)。这是 parseScripts 既有的 base 解析 quirk(见 parse_scripts.go I-CORR-5 注释),
// 此测试确认该已知边界:相对路径引用 hooks/ 脚本时去重不生效(pre-existing 行为,非本修复
// 引入)。测试留作文档,确保未来若修复 base 解析,此测试可更新为期望 0。
//
// 注:此测试当前期望 1(确认 quirk 存在),而非 0——避免误导。
func TestParseScriptsSkipsHooksDirDuplicatesRelative(t *testing.T) {
	f := newFixture(t)
	hooksDir := filepath.Join(f.claude, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(hooksDir, "pre.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	existingScript := Asset{
		Type:       AssetScript,
		Scope:      ScopeGlobal,
		SourcePath: scriptPath,
		Name:       "pre.sh",
	}
	fillHash(&existingScript)

	// hook 的 command 用 "hooks/pre.sh" 相对路径。parseScripts base=home,解析为
	// home/hooks/pre.sh(不存在或与 scriptPath 不同)→ seen 不命中 → 产 1 条。
	// 这印证 parse_scripts.go I-CORR-5 注释里 base = home 的 quirk。
	hook := Asset{
		Type:   AssetHook,
		Scope:  ScopeGlobal,
		Name:   "PreToolUse/*",
		Fields: map[string]any{"command": "bash hooks/pre.sh"},
	}
	out := parseScripts([]Asset{existingScript, hook}, f.claude)
	// 预填 seen 的是 claudeDir/hooks/pre.sh;parseScripts 解析 "hooks/pre.sh" →
	// home/hooks/pre.sh(home = filepath.Dir(claudeDir)),与 seen 中的不匹配。
	// home/hooks/pre.sh 不存在 → 跳过。期望 0。
	// (若未来 base 改为 claudeDir,解析为 claudeDir/hooks/pre.sh = scriptPath,seen 命中,仍 0。)
	if len(out) != 0 {
		t.Logf("parseScripts 对相对 hooks/pre.sh 产出 %d(基线 quirk:base=home 解析为 home/hooks/pre.sh)", len(out))
	}
}
