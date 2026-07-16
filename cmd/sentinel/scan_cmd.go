package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/history"
	"code-agent-sentinel/internal/scan"
	"code-agent-sentinel/internal/security"
)

// newScanCmd 构造 `sentinel scan` 子命令:一次性扫描(发现→扫描→写历史),
// 不启动 HTTP server。打印摘要(findings 数 / 耗时 / 健康分 / 不可用检测器)。
func newScanCmd() *cobra.Command {
	var cfgPath string
	var detectorsFlag string
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "执行一次性扫描并写入历史(不启动 HTTP server)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScanCmd(cmd, cfgPath, detectorsFlag)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "", "配置文件路径(默认 ~/.claude-sentinel/config.yaml)")
	cmd.Flags().StringVar(&detectorsFlag, "detectors", "", "只跑指定检测器(逗号分隔 ID;空=全量)")
	return cmd
}

// runScanCmd 执行一次性扫描。复用 scan.Runner(discover→scan→saveHistory),
// 检测器注册镜像 main.go run() / baseline_cmd runFullScan()。
func runScanCmd(cmd *cobra.Command, cfgPath, detectorsFlag string) error {
	cfg, home, err := loadCfgAndHome(cfgPath)
	if err != nil {
		return err
	}
	cfg.EnsureDetectors() // 与 main.go 一致:检测器持 cfg.Detectors 指针
	claudeDir := cfg.ResolveClaudeDir(home)
	eng := configengine.NewEngine(home, claudeDir)
	// 发现范围桥接(config 不导入 configengine,在此转 []AssetType)
	if cfg.Discovery != nil {
		for _, s := range cfg.Discovery.DisabledAssetTypes {
			eng.DisabledAssetTypes = append(eng.DisabledAssetTypes, configengine.AssetType(s))
		}
	}
	r := security.NewRegistry()
	r.Register(security.NewRulesDetector(home, cfg.Detectors))
	r.Register(security.NewSecretDetector(cfg.Detectors))
	r.Register(security.NewDependencyDetector(cfg.Detectors))
	orch := &security.Orchestrator{Registry: r}
	hist := history.NewStore(filepath.Join(home, ".claude-sentinel", "history"))
	runner := scan.NewRunner(eng, orch, hist)

	var ids []string
	if detectorsFlag != "" {
		ids = strings.Split(detectorsFlag, ",")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	res, err := runner.RunScan(ctx, ids)
	if err != nil {
		return fmt.Errorf("扫描失败: %w", err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "扫描完成: %d 条 finding,耗时 %s\n", len(res.Findings), res.Duration.Round(time.Millisecond))
	if res.HealthScore != nil {
		fmt.Fprintf(out, "健康分: %d (%s)\n", res.HealthScore.Score, res.HealthScore.Band)
	}
	for _, d := range res.Detectors {
		if !d.Available {
			fmt.Fprintf(out, "检测器 %s 不可用: %s\n", d.ID, d.Reason)
		}
	}
	return nil
}
