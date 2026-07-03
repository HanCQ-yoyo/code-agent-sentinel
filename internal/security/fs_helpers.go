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
