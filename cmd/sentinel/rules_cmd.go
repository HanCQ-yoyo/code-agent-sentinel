package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/security/ruleengine"
)

// newRulesCmd 构造 `sentinel rules` 子命令,含 list / validate [file]。
//
// rules list:列出所有已加载规则(builtin + global),含 id/severity/source/valid。
// rules validate [file]:校验单个规则文件(无参数则校验 builtin + global 全量)。
//
// 路径解析:读 config(默认 ~/.claude-sentinel/config.yaml),用 cfg.ResolveSentinelRulesDir(home)
// 解析全局规则目录。文件不存在 → 仅列/校验内置规则。
func newRulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "规则管理:list / validate",
	}
	cmd.AddCommand(newRulesListCmd())
	cmd.AddCommand(newRulesValidateCmd())
	return cmd
}

// loadRulesForCLI 加载 builtin + global 规则供 CLI 展示/校验。
// 返回 Validate 后的 valid 规则 + 全部加载/校验错误。
// 不加载项目规则(CLI 场景用户关心全局规则;项目规则在扫描时动态加载)。
func loadRulesForCLI(cfg *config.Config, home string) ([]ruleengine.Rule, []ruleengine.RuleLoadError) {
	builtin, errs := ruleengine.LoadBuiltin()
	globalDir := cfg.ResolveSentinelRulesDir(home)
	global, globalErrs := ruleengine.LoadDir(globalDir, "global:"+globalDir)
	errs = append(errs, globalErrs...)
	merged := ruleengine.Merge(builtin, global)
	valid, validateErrs := ruleengine.Validate(merged)
	errs = append(errs, validateErrs...)
	return valid, errs
}

func newRulesListCmd() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有已加载规则(id/severity/source/valid)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, home, err := loadCfgAndHome(cfgPath)
			if err != nil {
				return err
			}
			rules, loadErrs := loadRulesForCLI(cfg, home)
			out := cmd.OutOrStdout()

			fmt.Fprintf(out, "已加载 %d 条有效规则", len(rules))
			if len(loadErrs) > 0 {
				fmt.Fprintf(out, "(%d 条加载/校验错误)", len(loadErrs))
			}
			fmt.Fprintln(out)
			fmt.Fprintln(out)
			fmt.Fprintln(out, "ID                                                    SEVERITY  SOURCE                                          VALID")
			fmt.Fprintln(out, strings.Repeat("-", 120))
			for _, r := range rules {
				fmt.Fprintf(out, "%-52s %-9s %-46s %s\n", r.ID, r.Severity, r.Source, "yes")
			}
			for _, e := range loadErrs {
				id := e.RuleID
				if id == "" {
					id = "(parse error)"
				}
				fmt.Fprintf(out, "%-52s %-9s %-46s %s\n", id, "?", e.Source, "NO: "+truncateStr(e.Reason, 40))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "", "配置文件路径(默认 ~/.claude-sentinel/config.yaml)")
	return cmd
}

func newRulesValidateCmd() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:   "validate [file]",
		Short: "校验规则文件(无参数则校验全部已加载规则)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			// 单文件校验:用 LoadDir 加载该文件所在目录(只读 *.yaml)。
			// 若目录里有多个文件会全部加载,但 validate 的语义是"校验该文件",
			// 故限定只校验 args[0] 指定的文件:复制到临时目录单独加载。
			if len(args) == 1 {
				file := args[0]
				data, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("读取文件 %s: %w", file, err)
				}
				tmpDir, err := os.MkdirTemp("", "sentinel-validate-")
				if err != nil {
					return err
				}
				defer os.RemoveAll(tmpDir)
				dst := filepath.Join(tmpDir, filepath.Base(file))
				if err := os.WriteFile(dst, data, 0o644); err != nil {
					return err
				}
				rules, loadErrs := ruleengine.LoadDir(tmpDir, "file:"+file)
				// Validate:LoadDir 只解析 YAML,Validate 做 schema + 正则编译校验
				valid, validateErrs := ruleengine.Validate(rules)
				loadErrs = append(loadErrs, validateErrs...)
				return reportValidate(out, valid, loadErrs, file)
			}

			// 全量校验:builtin + global
			cfg, home, err := loadCfgAndHome(cfgPath)
			if err != nil {
				return err
			}
			rules, loadErrs := loadRulesForCLI(cfg, home)
			return reportValidate(out, rules, loadErrs, "builtin + global")
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "", "配置文件路径(默认 ~/.claude-sentinel/config.yaml)")
	return cmd
}

// reportValidate 打印校验结果。有 invalid 规则时返回 error(非零退出码)。
func reportValidate(out interface{ Write([]byte) (int, error) }, rules []ruleengine.Rule, loadErrs []ruleengine.RuleLoadError, scope string) error {
	fmt.Fprintf(out, "校验范围:%s\n", scope)
	fmt.Fprintf(out, "有效规则:%d 条\n", len(rules))
	for _, r := range rules {
		fmt.Fprintf(out, "  [valid] %s (severity=%s, source=%s)\n", r.ID, r.Severity, r.Source)
	}
	if len(loadErrs) == 0 {
		fmt.Fprintln(out, "校验通过:无错误。")
		return nil
	}
	fmt.Fprintf(out, "错误:%d 条:\n\n", len(loadErrs))
	for _, e := range loadErrs {
		id := e.RuleID
		if id == "" {
			id = "(未知规则)"
		}
		fmt.Fprintf(out, "  [%s] %s: %s\n", e.Source, id, e.Reason)
	}
	return fmt.Errorf("校验失败:%d 条规则有错误", len(loadErrs))
}

// loadCfgAndHome 加载配置与 home 目录(复用 run() 的逻辑,但不含 server 启动)。
func loadCfgAndHome(cfgPath string) (*config.Config, string, error) {
	if cfgPath == "" {
		p, err := config.DefaultPath()
		if err != nil {
			return nil, "", err
		}
		cfgPath = p
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, "", err
	}
	home := cfg.HomeDir
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, "", err
		}
		home = h
	}
	return cfg, home, nil
}

// truncateStr 截断字符串到 maxLen,超出加 "..."。
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
