//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

// forkChild 在 windows 平台 fork 一个脱离控制台的子进程。
// CREATE_NEW_PROCESS_GROUP(0x00000200) + DETACHED_PROCESS(0x00000008):
// 子进程不继承父进程的控制台,新进程组防止 Ctrl+C 信号传播。
func forkChild(childArgs []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, childArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008, // DETACHED_PROCESS
	}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}
