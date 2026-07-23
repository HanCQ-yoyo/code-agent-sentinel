package main

import (
	"fmt"
	"os"
)

// daemonize 实现后台启动(--daemon)。返回 (child, err):
//   - (true, nil): 当前进程是子进程(--daemon-child 在 argv 中),继续执行(不再 fork,防递归)。
//   - (false, nil): 当前进程是父进程,fork 成功,应立即退出。
//   - (false, err): fork 失败。
//
// argv 改造:去掉 --daemon 与 --daemon-child,追加 --daemon-child 标记子进程。
// 真正的 fork 由 build-tagged 的 forkChild 实现(见 fork_unix.go / fork_windows.go):
//   - linux:   Setsid(脱离会话)
//   - darwin:  Setpgid(Setsid 字段在 darwin syscall.SysProcAttr 中不存在)
//   - windows: CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS
func daemonize() (child bool, err error) {
	// 已是子进程(--daemon-child 在 argv 中)——防重复 fork。
	for _, arg := range os.Args[1:] {
		if arg == "--daemon-child" {
			return true, nil
		}
	}
	// 构造子进程 argv:去掉 --daemon / --daemon-child(防止重复),追加 --daemon-child。
	var childArgs []string
	for _, a := range os.Args[1:] {
		if a == "--daemon" || a == "--daemon-child" {
			continue
		}
		childArgs = append(childArgs, a)
	}
	childArgs = append(childArgs, "--daemon-child")
	if err := forkChild(childArgs); err != nil {
		return false, fmt.Errorf("fork 子进程失败: %w", err)
	}
	return false, nil
}
