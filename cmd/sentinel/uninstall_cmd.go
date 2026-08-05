package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newUninstallCmd() *cobra.Command {
	var yes bool
	var keepConfig bool
	var homeFlag string
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Clean up code-agent-sentinel data and config (does not delete ~/.claude or binary)",
		RunE: func(cmd *cobra.Command, args []string) error {
			home := homeFlag
			if home == "" {
				h, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				home = h
			}
			return runUninstall(home, yes, keepConfig, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip interactive confirmation")
	cmd.Flags().BoolVar(&keepConfig, "keep-config", false, "Keep config.yaml, only delete history/backups/baseline/suppressions")
	cmd.Flags().StringVar(&homeFlag, "home", "", "Override home directory (debug)")
	return cmd
}

// runUninstall 删除 ~/.code-agent-sentinel 数据目录。
// keepConfig=true 时保留 config.yaml,仅删 history/backups/baseline/suppressions。
// 安全:校验路径非根、非空、以 .code-agent-sentinel 结尾,且不是根的直接子目录
// (防 home="/" 解析出 /.code-agent-sentinel 后因目录不存在而静默返回 nil)。
func runUninstall(home string, yes, keepConfig bool, out io.Writer) error {
	dataDir := filepath.Clean(filepath.Join(home, ".code-agent-sentinel"))
	// 路径安全校验
	if dataDir == "/" || dataDir == "" {
		return fmt.Errorf("refused: data directory resolves to root or empty (%q)", dataDir)
	}
	if filepath.Base(dataDir) != ".code-agent-sentinel" {
		return fmt.Errorf("refused: data directory name is not .code-agent-sentinel (%q)", dataDir)
	}
	// 强化:拒绝根的直接子目录(如 home="/" → dataDir="/.code-agent-sentinel")。
	// 否则当该路径不存在时会落入下面的"目录不存在"分支静默返回 nil,
	// 违背"home 指向根 → 应拒绝"的测试意图。
	if filepath.Dir(dataDir) == "/" {
		return fmt.Errorf("refused: data directory is direct child of root, suspicious home pointing to root (%q)", dataDir)
	}
	info, err := os.Stat(dataDir)
	if os.IsNotExist(err) {
		fmt.Fprintf(out, "Directory does not exist, nothing to clean: %s\n", dataDir)
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("refused: %s is not a directory", dataDir)
	}

	// 整目录删除需 --yes 确认;keepConfig 路径只删子项(保留 config.yaml),同样实际删数据。
	// 两类实际删数据前先停并移除服务(若装过),避免删了数据服务还在跑报错。
	// 放在 --yes 预览(“将删除...添加 --yes”)之后:未确认的卸载只打印提示,不动服务。
	if keepConfig || yes {
		_ = runServiceUninstall(true) // best-effort:无服务时 systemctl 失败被忽略、os.Remove 不存在单元文件也忽略
		// Stage R2:移除运行时拦截 hook(~/.claude/settings.json,与数据目录分离,需单独清理)。
		// best-effort:settings 不存在视为已卸载,其他错误忽略(与 runServiceUninstall 一致的不阻塞策略)。
		UninstallGuardHook(filepath.Join(home, ".claude", "settings.json"))
		// Stage R3:移除 Codex 拦截 hook(~/.codex/hooks.json,与数据目录分离)。
		// best-effort:忽略错误,文件不存在视为已卸载(非 codex 用户根本没有此文件)。
		UninstallCodexHook(filepath.Join(home, ".codex", "hooks.json"))
	}

	if keepConfig {
		// 仅删子项(保留 config.yaml)
		targets := []string{"history", "backups", "baseline.json", "suppressions.yaml", "rules"}
		for _, name := range targets {
			p := filepath.Join(dataDir, name)
			if _, err := os.Stat(p); err == nil {
				if err := os.RemoveAll(p); err != nil {
					fmt.Fprintf(out, "Warning: failed to delete %s: %v\n", p, err)
				} else {
					fmt.Fprintf(out, "Deleted: %s\n", p)
				}
			}
		}
		fmt.Fprintf(out, "Kept config.yaml (keep-config)\n")
		return nil
	}

	// 整目录删除
	if !yes {
		fmt.Fprintf(out, "Will delete: %s\n", dataDir)
		fmt.Fprintf(out, "~/.claude and binary will not be deleted. Add --yes to confirm.\n")
		return nil
	}
	if err := os.RemoveAll(dataDir); err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}
	fmt.Fprintf(out, "Deleted: %s\n", dataDir)
	fmt.Fprintf(out, "To remove the binary, manually delete the code-agent-sentinel executable.\n")
	return nil
}
