package ruleengine

import (
	"strings"
	"testing"
)

func TestNormalizeStripSudo(t *testing.T) {
	cases := map[string]string{
		"sudo rm -rf /":           "rm -rf /",
		"sudo -u root rm -rf /":   "rm -rf /",
		"/usr/bin/sudo rm -rf /":  "rm -rf /", // basename 匹配
		"env VAR=1 rm -rf /":      "rm -rf /",
		"env -i rm -rf /":         "rm -rf /",
		"command rm -rf /":        "rm -rf /",
		"exec rm -rf /":           "rm -rf /",
		"nohup rm -rf /":          "rm -rf /",
		"\\rm -rf /":              "rm -rf /", // 反斜杠 alias bypass
		"sudo env X=1 command rm": "rm",
	}
	for in, want := range cases {
		if got := NormalizeCommand(in); got != want {
			t.Errorf("NormalizeCommand(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestNormalizeAnsiC(t *testing.T) {
	// $'\x72\x6d' = rm;只在 executable position 解码
	if got := NormalizeCommand("$'\\x72\\x6d' -rf /"); got != "rm -rf /" {
		t.Errorf("ANSI-C 解码不对: got %q", got)
	}
}

func TestNormalizePathExpand(t *testing.T) {
	if got := NormalizeCommand("/usr/bin/git reset --hard"); got != "git reset --hard" {
		t.Errorf("路径展开不对: got %q", got)
	}
}

func TestNormalizeQueryModeNotStripped(t *testing.T) {
	// command -v 是 query,不剥(dcg normalize.rs:735)
	if got := NormalizeCommand("command -v rm"); got != "command -v rm" {
		t.Errorf("command -v 不应被剥: got %q", got)
	}
}

func TestNormalizeDataAreaNotDecoded(t *testing.T) {
	// echo $'rm -rf /' 里 $'...' 是数据,不解码(dcg normalize.rs:2808)
	got := NormalizeCommand("echo $'rm -rf /'")
	if strings.Contains(got, "echo rm") {
		t.Errorf("数据区 ANSI-C 不应被解码: got %q", got)
	}
}
