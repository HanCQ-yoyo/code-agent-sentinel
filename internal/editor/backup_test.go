package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code-agent-sentinel/internal/configengine"
)

func TestBackupCreatesVersionedFile(t *testing.T) {
	home, claude := newFixture(t)
	src := filepath.Join(claude, "settings.json")
	writeFile(t, src, `{"model":"opus"}`)
	e := New(configengine.NewEngine(home), "", 0)
	a := configengine.Asset{Type: configengine.AssetSettings, SourcePath: src}
	bp, err := e.backup(a, []byte(`{"model":"opus"}`))
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	if bp == "" {
		t.Fatal("empty backup path")
	}
	if _, err := os.Stat(bp); err != nil {
		t.Fatalf("backup file not created: %v", err)
	}
	// 备份目录在 ~/.claude-sentinel/backups 下,不在 ~/.claude
	if !strings.HasPrefix(bp, e.BackupDir) {
		t.Fatalf("backup not under BackupDir: %q", bp)
	}
	// 备份文件不应落在 ~/.claude 目录树内(防污染源资产区)
	if strings.HasPrefix(bp, filepath.Join(home, ".claude")+string(filepath.Separator)) {
		t.Fatalf("backup must not live under ~/.claude: %q", bp)
	}
}

// TestBackupRollsOverMaxBackups 验证滚动裁剪:写入 > MaxBackups 份不同内容,
// 期望最终只保留 MaxBackups 份(最旧的被删除)。
//
// 修正说明:brief 原版循环写同一份内容,文件名 <ts>-<shorthash>.<ext> 中
// shorthash=sha256(content)[:8] 相同;time.Now() 毫秒精度在紧凑循环里可能产生
// 相同时间戳前缀,导致同名文件互相覆盖、count 永不超 MaxBackups,rollover 路径
// 从未被触发,`count > 3` 断言空转通过。改为每轮写不同内容 → 不同 shorthash →
// 文件名唯一(<ts>-<diff-shorthash>.json,即便时间戳相同也唯一)→ 累积超过
// MaxBackups → rollOver 真正删旧。断言 count == MaxBackups(恰好裁到上限)。
func TestBackupRollsOverMaxBackups(t *testing.T) {
	home, claude := newFixture(t)
	src := filepath.Join(claude, "settings.json")
	writeFile(t, src, `{"model":"opus"}`)
	e := New(configengine.NewEngine(home), "", 3) // MaxBackups=3
	a := configengine.Asset{Type: configengine.AssetSettings, SourcePath: src}
	for i := 0; i < 5; i++ {
		content := []byte(fmt.Sprintf(`{"model":"opus","n":%d}`, i))
		if _, err := e.backup(a, content); err != nil {
			t.Fatalf("backup %d: %v", i, err)
		}
	}
	// 收集该 source 的备份文件,应恰好 == MaxBackups(证明旧的被裁掉)
	dir := filepath.Join(e.BackupDir, sanitizePath(src))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read backup dir: %v", err)
	}
	count := 0
	for _, en := range entries {
		if !en.IsDir() && filepath.Ext(en.Name()) == ".json" {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("backups count = %d, want exactly MaxBackups(3) after rollover", count)
	}
	if count == 0 {
		t.Fatal("no backups after rollover")
	}
}

func TestBackupSanitizesPath(t *testing.T) {
	home, claude := newFixture(t)
	src := filepath.Join(claude, "sub", "deep", "settings.json")
	writeFile(t, src, `{"model":"opus"}`)
	e := New(configengine.NewEngine(home), "", 0)
	a := configengine.Asset{Type: configengine.AssetSettings, SourcePath: src}
	bp, err := e.backup(a, []byte("x"))
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	// sanitized 路径不含原始分隔符(防目录穿越),且备份落在 BackupDir 下
	rel, _ := filepath.Rel(e.BackupDir, bp)
	if strings.Contains(rel, "..") {
		t.Fatalf("backup escaped BackupDir: %q", rel)
	}
}
