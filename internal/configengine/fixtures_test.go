package configengine

import (
	"os"
	"path/filepath"
	"testing"
)

// fixtureHome 在临时目录里造一个假 ~/.claude 结构,返回 (homeDir, claudeJSONPath)。
type fixtureBuilder struct {
	home   string
	claude string // ~/.claude
	cj     string // ~/.claude.json
	t      *testing.T
}

func newFixture(t *testing.T) *fixtureBuilder {
	t.Helper()
	home := t.TempDir()
	claude := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	return &fixtureBuilder{home: home, claude: claude, cj: filepath.Join(home, ".claude.json"), t: t}
}

func (f *fixtureBuilder) write(rel string, content string) {
	f.t.Helper()
	p := filepath.Join(f.claude, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixtureBuilder) writeClaudeJSON(content string) {
	f.t.Helper()
	if err := os.WriteFile(f.cj, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// claudePath 返回 fixture 内 ~/.claude 下的绝对路径,供需要直接传路径给
// 解析函数(而非走 Engine.Discover)的测试使用。
func (f *fixtureBuilder) claudePath(rel string) string {
	return filepath.Join(f.claude, rel)
}

func (f *fixtureBuilder) writeProject(rel string, content string) {
	f.t.Helper()
	p := filepath.Join(f.home, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}
