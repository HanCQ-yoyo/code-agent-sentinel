package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/service"
)

// newServiceCmd 构造 `sentinel service` 父命令及其 install/uninstall/status 子命令。
// install 生成单元文件 + 写 config token +(非 dry-run)调 systemctl/launchctl/sc.exe 启用;
// uninstall/status 则 best-effort 调对应平台命令。
func newServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "管理系统服务(install/uninstall/status)",
	}
	var userMode bool
	var dryRun bool

	install := &cobra.Command{
		Use:   "install",
		Short: "安装 sentinel 为系统服务并启动",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			cfgPath, _ := config.DefaultPath()
			exe, _ := os.Executable()
			tok, err := runServiceInstall(serviceInstallOpts{
				Home: home, CfgPath: cfgPath, UserMode: userMode, DryRun: dryRun, ExePath: exe,
			})
			if err != nil {
				return err
			}
			fmt.Printf("token: %s\n", tok)
			return nil
		},
	}
	install.Flags().BoolVar(&userMode, "user", true, "用户级服务(无需 root)")
	install.Flags().BoolVar(&dryRun, "dry-run", false, "只生成单元文件,不执行 systemctl/launchctl")

	uninstall := &cobra.Command{
		Use:   "uninstall",
		Short: "停止并移除 sentinel 系统服务",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServiceUninstall(userMode)
		},
	}
	uninstall.Flags().BoolVar(&userMode, "user", true, "用户级服务")

	status := &cobra.Command{
		Use:   "status",
		Short: "查看 sentinel 服务状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServiceStatus(userMode)
		},
	}
	status.Flags().BoolVar(&userMode, "user", true, "用户级服务")

	cmd.AddCommand(install, uninstall, status)
	return cmd
}

// serviceInstallOpts 是 runServiceInstall 的参数集合,便于测试注入。
type serviceInstallOpts struct {
	Home     string
	CfgPath  string
	UserMode bool
	DryRun   bool
	ExePath  string
}

// runServiceInstall 生成单元文件 + 写 config token +(非 dry-run)调 systemctl/launchctl/sc。
// 返回 token(供调用方打印)。
//
// config.Load nil-handling:Load 在文件不存在时返回 (DefaultConfig(), nil)(见 config.go),
// 故 cfg 不会为 nil;此处的 `cfg == nil` 是对将来实现变更的防御性兜底。
func runServiceInstall(opts serviceInstallOpts) (string, error) {
	cfg, _ := config.Load(opts.CfgPath)
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	// token:config 已有则用之,否则生成写入
	tok := cfg.Token
	if tok == "" {
		tok = genToken()
		cfg.Token = tok
		if err := config.Save(opts.CfgPath, cfg); err != nil {
			return "", fmt.Errorf("写 config token 失败: %w", err)
		}
	}
	spec := service.UnitSpec{
		OS: runtime.GOOS, UserMode: opts.UserMode, Home: opts.Home,
		ExePath: opts.ExePath, Token: tok, Bind: cfg.Bind, Port: cfg.Port,
		LogPath: cfg.LogPath, // Task 14:用户配置 log_path 时,单元模板 StandardOutPath/StandardOutput 指向它(否则 launchd 回退默认 sentinel.log、systemd 走 journal)
	}
	unitPath, content, err := service.GenerateUnit(spec)
	if err != nil {
		return "", err
	}
	// 写单元文件(linux/macOS;windows 返回 unitPath="" 跳过)
	if unitPath != "" {
		if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(unitPath, []byte(content), 0o644); err != nil {
			return "", err
		}
	}
	if opts.DryRun {
		return tok, nil
	}
	// 实际启用:best-effort,错误忽略(用户可手动 systemctl status 查)
	switch runtime.GOOS {
	case "linux":
		// 条件构造 arg slice:UserMode 才加 --user,避免向 systemctl 传空串参数。
		args := daemonReloadArgs(opts.UserMode)
		exec.Command("systemctl", args...).Run()
		exec.Command("systemctl", append(enableNowArgs(opts.UserMode), "sentinel")...).Run()
	case "darwin":
		exec.Command("launchctl", "load", unitPath).Run()
	case "windows":
		// sc.exe 的 binPath= 值须紧跟等号后(空格分隔);含空格的路径(如 Program Files)须加引号,
		// 否则 sc.exe 按空格重切 argv 致 "syntax incorrect" 静默失败(与 service.go 生成器一致)。
		exe := scBinPathArg(opts.ExePath)
		exec.Command("sc.exe", "create", "sentinel", "binPath=", exe, "start=", "auto").Run()
		exec.Command("sc.exe", "start", "sentinel").Run()
	}
	return tok, nil
}

// daemonReloadArgs 构造 `systemctl [--user] daemon-reload` 的参数切片。
// UserMode=true 时含 --user;否则不含,避免向 systemctl 传空串参数。
func daemonReloadArgs(userMode bool) []string {
	if userMode {
		return []string{"--user", "daemon-reload"}
	}
	return []string{"daemon-reload"}
}

// enableNowArgs 构造 `systemctl [--user] enable --now` 的参数切片(不含单元名,由调用方 append)。
func enableNowArgs(userMode bool) []string {
	if userMode {
		return []string{"--user", "enable", "--now"}
	}
	return []string{"enable", "--now"}
}

// scBinPathArg 构造 sc.exe create 的 binPath= 值。sc.exe 要求值紧跟 `binPath= ` 之后
// (空格分隔),含空格的路径(如 `C:\Program Files\sentinel.exe`)须加双引号,否则
// sc.exe 按空格重切 argv 致 "The syntax of the command is incorrect." 静默失败
// (Go 的 exec 把 argv 用空格拼成命令行,裸传含空格路径等同未引用)。
// 与 internal/service/service.go generateWindows 的 `binPath= "%s"` 行为一致。
func scBinPathArg(exePath string) string {
	if strings.ContainsAny(exePath, " \t") {
		return `"` + exePath + `"`
	}
	return exePath
}

// runServiceUninstall 停止并移除 sentinel 系统服务。best-effort,错误忽略。
func runServiceUninstall(userMode bool) error {
	switch runtime.GOOS {
	case "linux":
		args := stopArgs(userMode)
		exec.Command("systemctl", args...).Run()
		exec.Command("systemctl", append(disableArgs(userMode), "sentinel")...).Run()
		var unitPath string
		if userMode {
			home, _ := os.UserHomeDir()
			unitPath = filepath.Join(home, ".config", "systemd", "user", "sentinel.service")
		} else {
			unitPath = "/etc/systemd/system/sentinel.service"
		}
		os.Remove(unitPath)
		exec.Command("systemctl", daemonReloadArgs(userMode)...).Run()
	case "darwin":
		home, _ := os.UserHomeDir()
		unitPath := filepath.Join(home, "Library", "LaunchAgents", "com.claude-sentinel.sentinel.plist")
		exec.Command("launchctl", "unload", unitPath).Run()
		os.Remove(unitPath)
	case "windows":
		exec.Command("sc.exe", "stop", "sentinel").Run()
		exec.Command("sc.exe", "delete", "sentinel").Run()
	}
	return nil
}

func stopArgs(userMode bool) []string {
	if userMode {
		return []string{"--user", "stop"}
	}
	return []string{"stop"}
}

func disableArgs(userMode bool) []string {
	if userMode {
		return []string{"--user", "disable"}
	}
	return []string{"disable"}
}

// runServiceStatus 查询 sentinel 服务状态,stdout/stderr 直连终端。best-effort。
func runServiceStatus(userMode bool) error {
	switch runtime.GOOS {
	case "linux":
		args := statusArgs(userMode)
		cmd := exec.Command("systemctl", append(args, "sentinel")...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	case "darwin":
		cmd := exec.Command("launchctl", "list", "com.claude-sentinel.sentinel")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	case "windows":
		cmd := exec.Command("sc.exe", "query", "sentinel")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}
	return nil
}

func statusArgs(userMode bool) []string {
	if userMode {
		return []string{"--user", "status"}
	}
	return []string{"status"}
}
