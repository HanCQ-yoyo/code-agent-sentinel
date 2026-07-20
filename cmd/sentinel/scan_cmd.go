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
	var agentFlag string
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "执行一次性扫描并写入历史(不启动 HTTP server)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScanCmd(cmd, cfgPath, detectorsFlag, agentFlag)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "", "配置文件路径(默认 ~/.claude-sentinel/config.yaml)")
	cmd.Flags().StringVar(&detectorsFlag, "detectors", "", "只跑指定检测器(逗号分隔 ID;空=全量)")
	cmd.Flags().StringVar(&agentFlag, "agent", "", "指定扫描的 code agent ID(空=回退首 agent)")
	return cmd
}

// runScanCmd 执行一次性扫描。复用 scan.Runner(discover→scan→saveHistory),
// 检测器注册镜像 main.go run() / baseline_cmd runFullScan()。
// 多 agent 解析镜像 main.go:ResolveAgents → 过滤 Enabled → AgentsFromSpecs → NewEngineFromAgent + NewRunner。
func runScanCmd(cmd *cobra.Command, cfgPath, detectorsFlag, agentFlag string) error {
	cfg, home, err := loadCfgAndHome(cfgPath)
	if err != nil {
		return err
	}
	cfg.EnsureDetectors() // 与 main.go 一致:检测器持 cfg.Detectors 指针
	// 多 agent:从 config 解析 enabled agents,桥接为 configengine.Agent(镜像 main.go run())。
	agentCfgs := cfg.ResolveAgents(home)
	agentItems := make([]configengine.AgentItem, 0, len(agentCfgs))
	for _, a := range agentCfgs {
		if a.Enabled {
			agentItems = append(agentItems, configengine.AgentItem{ID: a.ID, Enabled: a.Enabled, RootDir: a.RootDir, ClaudeJSON: a.ClaudeJSON})
		}
	}
	engAgents := configengine.AgentsFromSpecs(home, agentItems)
	if len(engAgents) == 0 {
		return fmt.Errorf("无启用的 code agent,运行 sentinel setup 配置")
	}
	// 本轮 Engine 仍取首个(Runner 内部按 agentID 池化,扫描时选)。
	eng := configengine.NewEngineFromAgent(engAgents[0])
	// 发现范围桥接(与 main.go 一致;NewEngineFromAgent 返回空 DisabledAssetTypes)。
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
	runner := scan.NewRunner(engAgents, orch, hist)

	var ids []string
	if detectorsFlag != "" {
		ids = strings.Split(detectorsFlag, ",")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	res, err := runner.RunScan(ctx, agentFlag, ids)
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
