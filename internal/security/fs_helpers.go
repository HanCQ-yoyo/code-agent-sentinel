package security

import "os"

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func hasGoMod(dir string) bool {
	fi, err := os.Stat(dir + string(os.PathSeparator) + "go.mod")
	return err == nil && !fi.IsDir()
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// truncate 截断字符串到 n 字符(超出加 "..."),用于 finding evidence 展示。
// secret/dependency/rules 检测器共用(原定义在 injection.go,Task 11 删 injection.go 后迁此)。
func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
