//go:build !windows

package main

import (
	"os"
	"os/exec"
	"runtime"
	"syscall"
)

// forkChild 在 unix/linux/macOS 平台 fork 一个脱离终端的子进程。
//   - linux:  SysProcAttr{Setsid: true} 创建新会话,脱离控制终端。
//   - darwin: SysProcAttr{Setpgid: true} 创建新进程组(darwin 的 syscall.SysProcAttr
//     没有 Setsid 字段,用 Setpgid 实现脱离终端会话——对 daemon 场景足够)。
//
// 子进程 stdin/stdout/stderr 全部置 nil(不继承父终端),Start 后父进程立即返回。
func forkChild(childArgs []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, childArgs...)
	var attr *syscall.SysProcAttr
	if runtime.GOOS == "darwin" {
		attr = &syscall.SysProcAttr{Setpgid: true}
	} else {
		attr = &syscall.SysProcAttr{Setsid: true}
	}
	cmd.SysProcAttr = attr
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}
