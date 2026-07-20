package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
)

// newSetupCmd 构造 `sentinel setup` 子命令:huh TUI 交互式配置 code agent。
// 流程:检测已知 agent → 勾选启用 → 逐个确认路径 → 预览 → 落盘到 config.yaml。
// 非 TTY 拒绝(管道/CI 无法交互);--allow-missing 旁路路径存在性校验。
// 可重跑:读现有 config,已检测到的 agent 默认勾选。
func newSetupCmd() *cobra.Command {
	var homeFlag, cfgPath string
	var allowMissing bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "交互式配置 code agent(选择启用的 agent + 确认路径)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(homeFlag, cfgPath, allowMissing, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&homeFlag, "home", "", "覆盖 home 目录(调试)")
	cmd.Flags().StringVar(&cfgPath, "config", "", "配置文件路径(默认 ~/.claude-sentinel/config.yaml)")
	cmd.Flags().BoolVar(&allowMissing, "allow-missing", false, "允许配置路径不存在的 agent(预配置未安装的 agent)")
	return cmd
}

// detectAgents 返回 Detect=true 的已知 agent 列表(纯函数,可测)。
// 不读真实 ~/.claude:home 由调用方注入。
func detectAgents(home string) []configengine.AgentSpec {
	var out []configengine.AgentSpec
	for _, s := range configengine.KnownAgents() {
		if s.Detect(home) {
			out = append(out, s)
		}
	}
	return out
}

// mergeAgents 把 setup 选择写入 cfg.Agents(保留其他字段,纯函数可测)。
func mergeAgents(cfg *config.Config, selection []config.AgentCfg) {
	cfg.Agents = selection
}

// runSetup 是 setup 主流程。非 TTY 报错;否则跑 huh 表单。
// in/out 显式传入便于测试(非 TTY 拒绝分支用管道模拟)。
// cobra 的 cmd.InOrStdin() 返回 io.Reader,故签名用 io.Reader/io.Writer;
// isTerminal 对 *os.File 做类型断言取 Fd()。
func runSetup(homeFlag, cfgPath string, allowMissing bool, in io.Reader, out io.Writer) error {
	home := homeFlag
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		home = h
	}
	// 非 TTY 拒绝(管道/CI 无法交互)。放在读 config 之前:失败不应有副作用。
	if !isTerminal(in) {
		return fmt.Errorf("setup 需交互式终端(当前 stdin 非 TTY);请在真实终端运行")
	}
	if cfgPath == "" {
		p, err := config.DefaultPath()
		if err != nil {
			return err
		}
		cfgPath = p
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	specs := configengine.KnownAgents()
	existing := detectAgents(home)
	// 默认勾选已检测到的 agent(可重跑:用户已装的默认开)。
	selectedIDs := make([]string, 0, len(existing))
	for _, s := range existing {
		selectedIDs = append(selectedIDs, s.ID)
	}

	// 第 1 屏:勾选启用哪些 agent。
	options := make([]huh.Option[string], len(specs))
	for i, s := range specs {
		options[i] = huh.NewOption(s.Name+" ("+s.ID+")", s.ID)
	}
	multi := huh.NewMultiSelect[string]().
		Title("选择要安全管控的 code agent").
		Options(options...).
		Value(&selectedIDs)
	if err := huh.NewForm(huh.NewGroup(multi)).Run(); err != nil {
		return err
	}
	if len(selectedIDs) == 0 {
		return fmt.Errorf("至少选择一个 agent")
	}

	// 第 2 屏起:逐个确认路径。
	selection := make([]config.AgentCfg, 0, len(selectedIDs))
	for _, id := range selectedIDs {
		var spec configengine.AgentSpec
		for _, s := range specs {
			if s.ID == id {
				spec = s
			}
		}
		rootDir := spec.DefaultRootDir(home)
		claudeJSON := spec.DefaultClaudeJSON(home)
		// NewGroup 接受 ...Field;构造 []huh.Field 以便变长展开。
		// *huh.Input 实现了 huh.Field 接口。
		fields := []huh.Field{huh.NewInput().Title(spec.Name + " 配置根目录").Value(&rootDir)}
		if spec.HasClaudeJSON {
			fields = append(fields, huh.NewInput().Title(spec.Name+" 机器管理文件").Value(&claudeJSON))
		}
		form := huh.NewForm(huh.NewGroup(fields...))
		if err := form.Run(); err != nil {
			return err
		}
		// 校验存在(--allow-missing 旁路)。
		if !allowMissing {
			if rootDir != "" {
				if _, err := os.Stat(rootDir); err != nil {
					return fmt.Errorf("%s 路径不存在: %s(用 --allow-missing 旁路)", spec.ID, rootDir)
				}
			}
			if spec.HasClaudeJSON && claudeJSON != "" {
				if _, err := os.Stat(claudeJSON); err != nil {
					return fmt.Errorf("%s 路径不存在: %s(用 --allow-missing 旁路)", spec.ID, claudeJSON)
				}
			}
		}
		selection = append(selection, config.AgentCfg{
			ID: id, Enabled: true, RootDir: rootDir, ClaudeJSON: claudeJSON,
		})
	}

	// 预览 + 确认。
	var sb strings.Builder
	sb.WriteString("将写入 agents:\n")
	for _, a := range selection {
		fmt.Fprintf(&sb, "  - id: %s\n    enabled: true\n    root_dir: %s\n    claude_json: %s\n",
			a.ID, a.RootDir, a.ClaudeJSON)
	}
	confirm := true
	cf := huh.NewConfirm().Title(sb.String() + "\n确认写入?").Value(&confirm)
	if err := huh.NewForm(huh.NewGroup(cf)).Run(); err != nil {
		return err
	}
	if !confirm {
		return fmt.Errorf("用户取消")
	}
	mergeAgents(cfg, selection)
	if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}
	fmt.Fprintf(out, "已写入 %s,重启 sentinel 生效\n", cfgPath)
	return nil
}

// isTerminal 判断 r 是否 TTY。golang.org/x/term 是标准轻量叶子包。
// r 可能是 *os.File(真实 stdin)或 *os.File(管道,非 TTY);做类型断言取 Fd()。
// 非 *os.File 一律视为非 TTY(保守拒绝)。
func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
