package editor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

func TestNewEditor(t *testing.T) {
	eng := configengine.NewEngine(t.TempDir())
	e := New(eng, "/tmp/backups", 20)
	if e == nil || e.Engine == nil {
		t.Fatal("New returned nil or nil Engine")
	}
	if e.BackupDir != "/tmp/backups" || e.MaxBackups != 20 {
		t.Fatalf("got BackupDir=%q MaxBackups=%d", e.BackupDir, e.MaxBackups)
	}
}

func TestNewEditorDefaults(t *testing.T) {
	eng := configengine.NewEngine(t.TempDir())
	e := New(eng, "", 0)
	if e.MaxBackups != 20 {
		t.Fatalf("default MaxBackups want 20 got %d", e.MaxBackups)
	}
	// BackupDir 空时默认 ~/.claude-sentinel/backups(home 之下 .claude-sentinel)
	want := filepath.Join(eng.HomeDir, ".claude-sentinel", "backups")
	if e.BackupDir != want {
		t.Fatalf("default BackupDir want %q got %q", want, e.BackupDir)
	}
}

func TestErrorsAreSentinels(t *testing.T) {
	if ErrConcurrentModification == nil || ErrNotEditable == nil || ErrOutOfRoot == nil || ErrBadContent == nil {
		t.Fatal("sentinel errors must be non-nil")
	}
}

func TestValidateContentJSONOk(t *testing.T) {
	e := New(configengine.NewEngine(t.TempDir()), "", 0)
	a := configengine.Asset{Type: configengine.AssetSettings}
	if err := e.validateContent(a, `{"model":"opus"}`); err != nil {
		t.Fatalf("valid JSON should pass: %v", err)
	}
}

func TestValidateContentJSONBad(t *testing.T) {
	e := New(configengine.NewEngine(t.TempDir()), "", 0)
	a := configengine.Asset{Type: configengine.AssetSettings}
	err := e.validateContent(a, `{not json`)
	if err != ErrBadContent {
		t.Fatalf("bad JSON want ErrBadContent got %v", err)
	}
}

func TestValidateContentMarkdownNotJSONChecked(t *testing.T) {
	e := New(configengine.NewEngine(t.TempDir()), "", 0)
	a := configengine.Asset{Type: configengine.AssetSkill}
	// markdown 不做 JSON 校验,任意文本通过
	if err := e.validateContent(a, "# title\nnot json {"); err != nil {
		t.Fatalf("markdown should not be JSON-validated: %v", err)
	}
}

func TestPreviewReadOnly(t *testing.T) {
	home, claude := newFixture(t)
	src := filepath.Join(claude, "settings.json")
	writeFile(t, src, `{"model":"opus"}`)
	e := New(configengine.NewEngine(home), "", 0)
	inv, _ := e.Engine.Discover()
	a := inv.Assets[0]
	// 读盘原始内容做 base_hash
	h, _, _ := configengine.HashAndMTime(src)
	pr, err := e.Preview(context.Background(), EditRequest{
		AssetID: a.ID, NewContent: `{"model":"sonnet"}`, BaseHash: h,
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !pr.Editable {
		t.Fatal("should be editable")
	}
	if !pr.BaseHashOK {
		t.Fatal("base hash should match")
	}
	if pr.Diff == "" {
		t.Fatal("diff should be non-empty")
	}
	// 盘上文件未被改
	got, _ := os.ReadFile(src)
	if string(got) != `{"model":"opus"}` {
		t.Fatalf("preview wrote to disk: %s", got)
	}
}

func TestPreviewNotEditable(t *testing.T) {
	home, _ := newFixture(t)
	// 伪造不可编辑资产 ID(指向 ~/.claude.json 的 MCP)
	writeFile(t, filepath.Join(home, ".claude.json"), `{"mcpServers":{"foo":{"command":"bar"}}}`)
	e := New(configengine.NewEngine(home), "", 0)
	inv, _ := e.Engine.Discover()
	var rogue configengine.Asset
	for _, a := range inv.Assets {
		if a.SourcePath == filepath.Join(home, ".claude.json") {
			rogue = a
			break
		}
	}
	if rogue.ID == "" {
		t.Fatal("no rogue asset")
	}
	pr, err := e.Preview(context.Background(), EditRequest{AssetID: rogue.ID, NewContent: `{}`, BaseHash: "x"})
	if err != nil {
		t.Fatalf("preview err: %v", err)
	}
	if pr.Editable {
		t.Fatal("~/.claude.json asset should not be editable")
	}
}

func TestCommitWritesAndBacksUp(t *testing.T) {
	home, claude := newFixture(t)
	src := filepath.Join(claude, "settings.json")
	writeFile(t, src, `{"model":"opus"}`)
	e := New(configengine.NewEngine(home), "", 0)
	inv, _ := e.Engine.Discover()
	a := inv.Assets[0]
	h, _, _ := configengine.HashAndMTime(src)
	res, err := e.Commit(context.Background(), EditRequest{
		AssetID: a.ID, NewContent: `{"model":"sonnet"}`, BaseHash: h,
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	// 盘上已是新内容
	got, _ := os.ReadFile(src)
	if string(got) != `{"model":"sonnet"}` {
		t.Fatalf("disk not updated: %s", got)
	}
	// 备份存在
	if res.BackupPath == "" {
		t.Fatal("no backup path")
	}
	if _, err := os.Stat(res.BackupPath); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	// 备份内容是旧内容
	bk, _ := os.ReadFile(res.BackupPath)
	if string(bk) != `{"model":"opus"}` {
		t.Fatalf("backup should hold old content: %s", bk)
	}
	// 新 hash 重算且非空
	if res.Asset.Hash == "" || res.Asset.Hash == h {
		t.Fatal("new hash not recomputed")
	}
}

func TestCommitConcurrentModification(t *testing.T) {
	home, claude := newFixture(t)
	src := filepath.Join(claude, "settings.json")
	writeFile(t, src, `{"model":"opus"}`)
	e := New(configengine.NewEngine(home), "", 0)
	inv, _ := e.Engine.Discover()
	a := inv.Assets[0]
	// 盘上被 Claude Code 改了
	writeFile(t, src, `{"model":"haiku"}`)
	_, err := e.Commit(context.Background(), EditRequest{
		AssetID: a.ID, NewContent: `{"model":"sonnet"}`, BaseHash: "stale-hash",
	})
	if err != ErrConcurrentModification {
		t.Fatalf("want ErrConcurrentModification got %v", err)
	}
}

func TestCommitBadContent(t *testing.T) {
	home, claude := newFixture(t)
	src := filepath.Join(claude, "settings.json")
	writeFile(t, src, `{"model":"opus"}`)
	e := New(configengine.NewEngine(home), "", 0)
	inv, _ := e.Engine.Discover()
	a := inv.Assets[0]
	h, _, _ := configengine.HashAndMTime(src)
	_, err := e.Commit(context.Background(), EditRequest{
		AssetID: a.ID, NewContent: `{not json`, BaseHash: h,
	})
	if err != ErrBadContent {
		t.Fatalf("want ErrBadContent got %v", err)
	}
}

func TestCommitNotEditable(t *testing.T) {
	home, _ := newFixture(t)
	writeFile(t, filepath.Join(home, ".claude.json"), `{"mcpServers":{"foo":{"command":"bar"}}}`)
	e := New(configengine.NewEngine(home), "", 0)
	inv, _ := e.Engine.Discover()
	var rogue configengine.Asset
	for _, a := range inv.Assets {
		if a.SourcePath == filepath.Join(home, ".claude.json") {
			rogue = a
			break
		}
	}
	_, err := e.Commit(context.Background(), EditRequest{AssetID: rogue.ID, NewContent: `{}`, BaseHash: "x"})
	if err != ErrNotEditable {
		t.Fatalf("want ErrNotEditable got %v", err)
	}
}
