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
		Short: "清理 sentinel 历史数据与配置(不删 ~/.claude 与二进制)",
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
	cmd.Flags().BoolVar(&yes, "yes", false, "跳过交互确认")
	cmd.Flags().BoolVar(&keepConfig, "keep-config", false, "保留 config.yaml,仅删 history/backups/baseline/suppressions")
	cmd.Flags().StringVar(&homeFlag, "home", "", "覆盖 home 目录(调试用)")
	return cmd
}

// runUninstall 删除 ~/.claude-sentinel 数据目录。
// keepConfig=true 时保留 config.yaml,仅删 history/backups/baseline/suppressions。
// 安全:校验路径非根、非空、以 .claude-sentinel 结尾,且不是根的直接子目录
// (防 home="/" 解析出 /.claude-sentinel 后因目录不存在而静默返回 nil)。
func runUninstall(home string, yes, keepConfig bool, out io.Writer) error {
	dataDir := filepath.Clean(filepath.Join(home, ".claude-sentinel"))
	// 路径安全校验
	if dataDir == "/" || dataDir == "" {
		return fmt.Errorf("拒绝:数据目录解析为根或空(%q)", dataDir)
	}
	if filepath.Base(dataDir) != ".claude-sentinel" {
		return fmt.Errorf("拒绝:数据目录名不是 .claude-sentinel(%q)", dataDir)
	}
	// 强化:拒绝根的直接子目录(如 home="/" → dataDir="/.claude-sentinel")。
	// 否则当该路径不存在时会落入下面的"目录不存在"分支静默返回 nil,
	// 违背"home 指向根 → 应拒绝"的测试意图。
	if filepath.Dir(dataDir) == "/" {
		return fmt.Errorf("拒绝:数据目录是根的直接子目录,疑似 home 指向根(%q)", dataDir)
	}
	info, err := os.Stat(dataDir)
	if os.IsNotExist(err) {
		fmt.Fprintf(out, "目录不存在,无需清理:%s\n", dataDir)
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("拒绝:%s 不是目录", dataDir)
	}

	// 整目录删除需 --yes 确认;keepConfig 路径只删子项(保留 config.yaml),同样实际删数据。
	// 两类实际删数据前先停并移除服务(若装过),避免删了数据服务还在跑报错。
	// 放在 --yes 预览(“将删除...添加 --yes”)之后:未确认的卸载只打印提示,不动服务。
	if keepConfig || yes {
		_ = runServiceUninstall(true) // best-effort:无服务时 systemctl 失败被忽略、os.Remove 不存在单元文件也忽略
	}

	if keepConfig {
		// 仅删子项(保留 config.yaml)
		targets := []string{"history", "backups", "baseline.json", "suppressions.yaml", "rules"}
		for _, name := range targets {
			p := filepath.Join(dataDir, name)
			if _, err := os.Stat(p); err == nil {
				if err := os.RemoveAll(p); err != nil {
					fmt.Fprintf(out, "警告:删除 %s 失败: %v\n", p, err)
				} else {
					fmt.Fprintf(out, "已删除:%s\n", p)
				}
			}
		}
		fmt.Fprintf(out, "已保留 config.yaml(keep-config)\n")
		return nil
	}

	// 整目录删除
	if !yes {
		fmt.Fprintf(out, "将删除:%s\n", dataDir)
		fmt.Fprintf(out, "~/.claude 与二进制不会被删。添加 --yes 确认执行。\n")
		return nil
	}
	if err := os.RemoveAll(dataDir); err != nil {
		return fmt.Errorf("删除失败: %w", err)
	}
	fmt.Fprintf(out, "已删除:%s\n", dataDir)
	fmt.Fprintf(out, "如需删除二进制,请手动 rm sentinel 可执行文件。\n")
	return nil
}
