package editor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"code-agent-sentinel/internal/configengine"
)

// backup 把 content 作为 a.SourcePath 的一份版本化备份写入 BackupDir,返回备份路径。
// 路径:<BackupDir>/<sanitized-sourcepath>/<ts>-<shorthash>.<ext>。滚动裁剪到 MaxBackups。
// 目录 0o700、文件 0o600;BackupDir 默认 <home>/.claude-sentinel/backups,不污染 ~/.claude/。
func (e *Editor) backup(a configengine.Asset, content []byte) (string, error) {
	sub := sanitizePath(a.SourcePath)
	dir := filepath.Join(e.BackupDir, sub)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	ts := nowTimestamp()
	shorthash := sha256hex(content)[:8]
	ext := filepath.Ext(a.SourcePath)
	name := fmt.Sprintf("%s-%s%s", ts, shorthash, ext)
	bp := filepath.Join(dir, name)
	if err := os.WriteFile(bp, content, 0o600); err != nil {
		return "", err
	}
	if err := e.rollOver(dir, ext); err != nil {
		// 裁剪失败不阻断备份本身(已写成功),仅记
		_ = err
	}
	return bp, nil
}

// rollOver 删除 dir 下最旧的备份,保留 MaxBackups 份。
// 按文件名排序(ts 前缀字典序=时间序;同 ts 时 shorthash 区分,顺序无意义只保证确定性)。
func (e *Editor) rollOver(dir, ext string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, en := range entries {
		if en.IsDir() || filepath.Ext(en.Name()) != ext {
			continue
		}
		names = append(names, en.Name())
	}
	sort.Strings(names) // ts 前缀字典序
	for len(names) > e.MaxBackups {
		oldest := names[0]
		names = names[1:]
		if err := os.Remove(filepath.Join(dir, oldest)); err != nil {
			return err
		}
	}
	return nil
}

// sanitizePath 把绝对路径转成安全的单层目录名:分隔符全替成 _,去掉前导分隔符,
// 并清除 .. 残留(防目录穿越)。同 source 的备份聚到同一子目录。
func sanitizePath(p string) string {
	s := filepath.Clean(p)
	s = strings.TrimPrefix(s, string(filepath.Separator))
	s = strings.ReplaceAll(s, string(filepath.Separator), "_")
	// 去 .. 残留(防御)
	s = strings.ReplaceAll(s, "..", "_")
	return s
}

// sha256hex 返回 content 的 sha256 十六进制摘要。
func sha256hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// nowTimestamp 返回备份文件名用的时间戳(年月日时分秒+毫秒,字典序=时间序)。
// CLAUDE.md 的 time 限制只针对 workflow 脚本,Go 代码/测试可用 time.Now()。
func nowTimestamp() string {
	return time.Now().Format("20060102-150405.000")
}
