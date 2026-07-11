package configengine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMarkdownDir(t *testing.T) {
	f := newFixture(t)
	f.write("skills/foo/SKILL.md", "---\nname: foo\ndescription: d\n---\nbody text")
	f.write("skills/bar.md", "no frontmatter")
	assets, err := parseMarkdownDir(filepath.Join(f.claude, "skills"), AssetSkill, ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 2 {
		t.Fatalf("want 2, got %d", len(assets))
	}
	names := map[string]bool{}
	for _, a := range assets {
		names[a.Name] = true
		if a.Content == "" {
			t.Errorf("%s 无 content", a.Name)
		}
		if a.Hash == "" {
			t.Errorf("%s 无 hash", a.Name)
		}
		if a.ID == "" {
			t.Errorf("%s 无 id", a.Name)
		}
		if a.Scope != ScopeGlobal {
			t.Errorf("%s scope 不是 global: %s", a.Name, a.Scope)
		}
	}
	if !names["foo"] || !names["bar"] {
		t.Errorf("names: %v", names)
	}
	// foo 带 frontmatter,应解析出 name/description 字段。
	for _, a := range assets {
		if a.Name == "foo" {
			if a.Fields["name"] != "foo" {
				t.Errorf("foo fields.name = %v, want foo", a.Fields["name"])
			}
			if a.Fields["description"] != "d" {
				t.Errorf("foo fields.description = %v, want d", a.Fields["description"])
			}
			if a.Content != "body text" {
				t.Errorf("foo content = %q, want %q", a.Content, "body text")
			}
		}
	}
}

func TestParseMarkdownDirMissingDir(t *testing.T) {
	f := newFixture(t)
	// 目录不存在:返回 nil, nil(不致错)。
	assets, err := parseMarkdownDir(filepath.Join(f.claude, "nope"), AssetSkill, ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	if assets != nil {
		t.Errorf("want nil assets for missing dir, got %d", len(assets))
	}
}

func TestParseSkillAllowedTools(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	os.MkdirAll(skillDir, 0o755)
	content := "---\nname: my-skill\ndescription: test\nallowed-tools: Bash,Read(*)\n---\nbody\n"
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644)

	assets, err := parseMarkdownDir(dir, AssetSkill, ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 {
		t.Fatalf("want 1 asset, got %d", len(assets))
	}
	at, ok := assets[0].Fields["allowed-tools"]
	if !ok {
		t.Fatal("allowed-tools 字段未解析进 Fields")
	}
	s := fmt.Sprint(at)
	if !strings.Contains(s, "Bash") {
		t.Errorf("allowed-tools = %q, want contain Bash", s)
	}
}

func TestParseMemory(t *testing.T) {
	f := newFixture(t)
	f.write("CLAUDE.md", "# project memory\nsome notes")
	f.write("memory/note1.md", "note one")
	f.write("memory/note2.md", "note two")
	assets, err := parseMemory(f.claude, ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 3 {
		t.Fatalf("want 3 (CLAUDE.md + 2 memory), got %d", len(assets))
	}
	names := map[string]bool{}
	for _, a := range assets {
		names[a.Name] = true
		if a.Type != AssetMemory {
			t.Errorf("%s type = %s, want memory", a.Name, a.Type)
		}
		if a.Content == "" {
			t.Errorf("%s 无 content", a.Name)
		}
		if a.Hash == "" {
			t.Errorf("%s 无 hash", a.Name)
		}
		if a.ID == "" {
			t.Errorf("%s 无 id", a.Name)
		}
	}
	if !names["CLAUDE.md"] || !names["note1.md"] || !names["note2.md"] {
		t.Errorf("names: %v", names)
	}
}

func TestParseMemoryMissing(t *testing.T) {
	f := newFixture(t)
	// 无 CLAUDE.md 也无 memory/:返回 nil/nil。
	assets, err := parseMemory(f.claude, ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	if assets != nil {
		t.Errorf("want nil assets, got %d", len(assets))
	}
}
