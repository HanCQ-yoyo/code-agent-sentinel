package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security"
	"code-agent-sentinel/internal/security/findingstate"
)

// newBaselineCmd 构造 `sentinel baseline` 子命令(--create / --prune)。
//
// Task 11 语义变更:旧实现写 baseline.json(已删);新实现统一到 finding_states.yaml:
//   - --create:跑全量扫描,把所有 Finding 的 fingerprint 批量接受(accepted),与 API POST /api/baseline 一致。
//   - --prune:重新扫描,删掉 finding_states.yaml 中已不复现的孤儿状态(PruneReport + Remove)。
//
// 路径解析:finding_states.yaml 在 <home>/.claude-sentinel/(不接 config 覆盖,统一默认路径)。
// 扫描逻辑镜像 main.go run():构建 Engine + Registry + Orchestrator,跑全量 Scan。
func newBaselineCmd() *cobra.Command {
	var (
		cfgPath string
		create  bool
		prune   bool
	)
	cmd := &cobra.Command{
		Use:   "baseline",
		Short: "baseline 管理:--create 批量接受 / --prune 清理孤儿状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !create && !prune {
				return fmt.Errorf("请指定 --create 或 --prune")
			}
			cfg, home, err := loadCfgAndHome(cfgPath)
			if err != nil {
				return err
			}
			if create {
				return runBaselineCreateCmd(cmd, cfg, home)
			}
			return runBaselinePruneCmd(cmd, cfg, home)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "", "配置文件路径(默认 ~/.claude-sentinel/config.yaml)")
	cmd.Flags().BoolVar(&create, "create", false, "跑全量扫描并把指纹批量接受到 finding_states.yaml")
	cmd.Flags().BoolVar(&prune, "prune", false, "重新扫描并删除 finding_states.yaml 中已不复现的孤儿状态")
	return cmd
}

// runFullScan 镜像 main.go run() 的扫描设置,跑一次全量扫描返回 findings。
// 不启动 HTTP server。用传入的 cfg 解析路径(供未来 detector 读 cfg)。
func runFullScan(cfg *config.Config, home string) (*security.ScanResult, error) {
	claudeDir := cfg.ResolveClaudeDir(home)
	eng := configengine.NewEngine(home, claudeDir)
	// #2:发现范围桥接(config 不导入 configengine,在此转 []AssetType)
	if cfg.Discovery != nil {
		for _, s := range cfg.Discovery.DisabledAssetTypes {
			eng.DisabledAssetTypes = append(eng.DisabledAssetTypes, configengine.AssetType(s))
		}
	}
	inv, err := eng.Discover()
	if err != nil {
		return nil, fmt.Errorf("发现资产失败: %w", err)
	}
	cfg.EnsureDetectors() // 与 main.go 一致:检测器持 cfg.Detectors 指针
	r := security.NewRegistry()
	r.Register(security.NewRulesDetector(home, cfg.Detectors))
	r.Register(security.NewSecretDetector(cfg.Detectors))
	r.Register(security.NewDependencyDetector(cfg.Detectors))
	orch := &security.Orchestrator{Registry: r}
	res, err := orch.Scan(context.Background(), inv.Assets, nil)
	if err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
	}
	return res, nil
}

// collectFingerprints 从扫描结果收集所有非空 fingerprint(去重)。
// 仅 RulesDetector 的 finding 带 fingerprint(parse.error 等兜底 finding 无)。
func collectFingerprints(res *security.ScanResult) []string {
	seen := make(map[string]bool)
	for _, f := range res.Findings {
		if f.Fingerprint != "" {
			seen[f.Fingerprint] = true
		}
	}
	fps := make([]string, 0, len(seen))
	for fp := range seen {
		fps = append(fps, fp)
	}
	return fps
}

// runBaselineCreateCmd 执行 --create:全量扫描 → 批量接受 fingerprint → 保存 finding_states.yaml。
func runBaselineCreateCmd(cmd *cobra.Command, cfg *config.Config, home string) error {
	out, err := runBaselineCreate(cfg, home)
	fmt.Fprint(cmd.OutOrStdout(), out)
	return err
}

// runBaselineCreate 跑全量扫描,把全部 Finding 的 fingerprint 批量接受到 finding_states.yaml。
// 已有非 open 状态(resolved/false_positive/accepted/in_progress)不覆盖,尊重既有处置。
// 返回可读输出。
func runBaselineCreate(cfg *config.Config, home string) (string, error) {
	res, err := runFullScan(cfg, home)
	if err != nil {
		return "", err
	}
	fps := collectFingerprints(res)
	statesPath := statesPathFor(home)

	st, _ := findingstate.Load(statesPath)
	if st == nil {
		st = &findingstate.States{}
	}
	st.BulkAccept(fps, findingstate.SourceBulkAccept, time.Now().UTC().Format(time.RFC3339))
	if err := st.Save(statesPath); err != nil {
		return "", fmt.Errorf("保存 finding_states 失败: %w", err)
	}
	return fmt.Sprintf("处置状态已批量接受: %s\n  扫描产出 %d 条 finding, 批量接受 %d 条指纹\n",
		statesPath, len(res.Findings), len(fps)), nil
}

// runBaselinePruneCmd 执行 --prune:加载 finding_states → 重新扫描 → 删已不复现指纹 → 保存。
func runBaselinePruneCmd(cmd *cobra.Command, cfg *config.Config, home string) error {
	out, err := runBaselinePrune(cfg, home)
	fmt.Fprint(cmd.OutOrStdout(), out)
	return err
}

// runBaselinePrune 重新扫描,删除 finding_states.yaml 中已不复现的指纹。
// 返回可读输出。
func runBaselinePrune(cfg *config.Config, home string) (string, error) {
	statesPath := statesPathFor(home)
	st, err := findingstate.Load(statesPath)
	if err != nil {
		return "", fmt.Errorf("加载 finding_states 失败: %w", err)
	}
	if st == nil {
		return "", fmt.Errorf("finding_states 文件不存在: %s(请先 --create)", statesPath)
	}
	res, err := runFullScan(cfg, home)
	if err != nil {
		return "", err
	}
	current := collectFingerprints(res)

	// 用 PruneReport 找出孤儿(已处置但本轮未检出),逐条 Remove。
	orphans := st.PruneReport(current)
	for _, o := range orphans {
		st.Remove(o.Fingerprint)
	}
	if err := st.Save(statesPath); err != nil {
		return "", fmt.Errorf("保存 pruned finding_states 失败: %w", err)
	}
	remain := len(st.Items)
	return fmt.Sprintf("处置状态已清理: %s\n  保留 %d 条, 删除 %d 条已不复现\n",
		statesPath, remain, len(orphans)), nil
}

// statesPathFor 返回 <home>/.claude-sentinel/finding_states.yaml。
func statesPathFor(home string) string {
	return config.DefaultConfig().ResolveStatesPath(home)
}
