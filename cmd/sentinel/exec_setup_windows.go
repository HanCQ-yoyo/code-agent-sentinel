//go:build windows

package main

import (
	"os"
	"os/exec"
)

// execSetup 启动 setup 子命令前台运行，阻塞等待完成后以相同退出码退出。
// windows 不支持 syscall.Exec 替换进程映像，改用子进程 + 等待 + 退出。
func execSetup(exe string) error {
	cmd := exec.Command(exe, "setup")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// 如果子进程因非 0 码退出（退出码 > 0），直接退出当前进程。
		// cmd.Run() 返回 *exec.ExitError 时退出码 > 0。
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		return err
	}
	// setup 正常结束（退出码 0），也退出当前进程。
	os.Exit(0)
	return nil
}
