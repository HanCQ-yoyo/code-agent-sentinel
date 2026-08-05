//go:build !windows

package main

import (
	"os"
	"syscall"
)

// execSetup 替换当前进程为 setup 子命令（保留 TTY 交互）。
// unix 平台使用 syscall.Exec 原地替换当前进程镜像，不创建新 PID。
func execSetup(exe string) error {
	return syscall.Exec(exe, []string{exe, "setup"}, os.Environ())
}
