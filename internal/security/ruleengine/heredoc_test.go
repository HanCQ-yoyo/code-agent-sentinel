package ruleengine

import "testing"

func TestHasInlineScriptTriggers(t *testing.T) {
	positive := []string{
		`python -c "import os; os.system('rm -rf /')"`,
		`bash -c "rm -rf /"`,
		`sh -c 'rm -rf /'`,
		`node -e "require('fs').rmSync('/')"`,
		`perl -e 'unlink("/")'`,
		`eval 'rm -rf /'`,
		`echo rm -rf / | bash`,
		`cat <<< "rm -rf /"`,
		`cmd /c "del /s *"`,
	}
	for _, c := range positive {
		if !HasInlineScript(c) {
			t.Errorf("HasInlineScript(%q)=false, want true", c)
		}
	}
	negative := []string{
		"rm -rf /", "git reset --hard", "ls -la", "echo hello",
	}
	for _, c := range negative {
		if HasInlineScript(c) {
			t.Errorf("HasInlineScript(%q)=true, want false", c)
		}
	}
}

func TestExtractInlineScriptBashC(t *testing.T) {
	got := ExtractInlineScripts(`bash -c "rm -rf /"`, 0)
	if len(got) != 1 || got[0] != "rm -rf /" {
		t.Fatalf("应提取内层 rm -rf /, got %v", got)
	}
}

func TestExtractInlineScriptNested(t *testing.T) {
	// bash -c "$(echo rm -rf /)" — Tier2.5 手写分段递归
	got := ExtractInlineScripts(`bash -c "$(echo rm -rf /)"`, 0)
	found := false
	for _, c := range got {
		if c == "rm -rf /" || containsStr(c, "rm -rf /") {
			found = true
		}
	}
	if !found {
		t.Fatalf("嵌套应递归提取到 rm -rf /, got %v", got)
	}
}

func TestExtractInlineScriptDepthLimit(t *testing.T) {
	// 深度上限 8,超过不递归(防无限递归)
	deep := `bash -c "bash -c rm -rf /"`
	got := ExtractInlineScripts(deep, MaxEmbeddedShellDepth)
	if len(got) == 0 {
		t.Fatal("深度上限内应至少提取一层")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
