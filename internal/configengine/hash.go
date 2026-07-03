package configengine

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"time"
)

// HashAndMTime 返回文件内容的 sha256(十六进制)与修改时间。
func HashAndMTime(path string) (string, time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", time.Time{}, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", time.Time{}, err
	}
	fi, err := f.Stat()
	if err != nil {
		return "", time.Time{}, err
	}
	return hex.EncodeToString(h.Sum(nil)), fi.ModTime(), nil
}
