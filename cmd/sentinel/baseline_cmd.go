package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security"
	"code-agent-sentinel/internal/security/suppression"
)

// newBaselineCmd 构造 `sentinel baseline` 子命令(--create / --prune)。
//
// --create:跑一次全量扫描,把所有 Finding 的 fingerprint 合并(union)到 baseline.json
// (保留已有指纹 + 添加新发现,与 API POST /api/baseline 一致;不清理不复现指纹)。
// --prune:重新扫描,删掉 baseline 中已不复现的指纹,保存。
//
// 路径解析:baseline.json 路径来自 cfg.ResolveBaselinePath(home)(支持 config 覆盖)。
// 扫描逻辑镜像 main.go run():构建 Engine + Registry + Orchestrator,跑全量 Scan。
func newBaselineCmd() *cobra.Command {
	var (
		cfgPath string
		create  bool
		prune   bool
	)
	cmd := &cobra.Command{
		Use:   "baseline",
		Short: "baseline 管理:--create 合并快照 / --prune 清理",
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
	cmd.Flags().BoolVar(&create, "create", false, "跑全量扫描并把指纹合并到 baseline.json(保留已有 + 添加新发现)")
	cmd.Flags().BoolVar(&prune, "prune", false, "重新扫描并删除 baseline 中已不复现的指纹")
	return cmd
}

// runFullScan 镜像 main.go run() 的扫描设置,跑一次全量扫描返回 findings。
// 不启动 HTTP server。用传入的 cfg 解析路径(供未来 detector 读 cfg)。
func runFullScan(cfg *config.Config, home string) (*security.ScanResult, error) {
	eng := configengine.NewEngine(home)
	inv, err := eng.Discover()
	if err != nil {
		return nil, fmt.Errorf("发现资产失败: %w", err)
	}
	r := security.NewRegistry()
	r.Register(security.NewRulesDetector(home, nil))
	r.Register(security.NewSecretDetector(nil))
	r.Register(security.NewDependencyDetector(nil))
	orch := &security.Orchestrator{Registry: r}
	res, err := orch.Scan(context.Background(), inv.Assets, nil)
	if err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
	}
	return res, nil
}

// collectFingerprints 从扫描结果收集所有非空 fingerprint。
// 仅 RulesDetector 的 finding 带 fingerprint(parse.error 等兜底 finding 无)。
func collectFingerprints(res *security.ScanResult) map[string]bool {
	fps := make(map[string]bool)
	for _, f := range res.Findings {
		if f.Fingerprint != "" {
			fps[f.Fingerprint] = true
		}
	}
	return fps
}

// runBaselineCreateCmd 执行 --create:全量扫描 → 快照指纹 → 保存 baseline.json。
func runBaselineCreateCmd(cmd *cobra.Command, cfg *config.Config, home string) error {
	out, err := runBaselineCreate(cfg, home)
	fmt.Fprint(cmd.OutOrStdout(), out)
	return err
}

// runBaselineCreate 跑全量扫描,把全部 Finding 的 fingerprint 合并到 baseline.json。
// Finding #2:UNION 语义(保留已有指纹 + 添加新发现),与 API postBaseline 一致。
// 旧实现用当前扫描的指纹直接覆盖,会丢失之前记录但当前不复现的指纹(覆盖语义)。
// 返回可读输出。路径由 cfg.ResolveBaselinePath(home) 解析(支持 config 覆盖)。
func runBaselineCreate(cfg *config.Config, home string) (string, error) {
	res, err := runFullScan(cfg, home)
	if err != nil {
		return "", err
	}
	fps := collectFingerprints(res)
	baselinePath := cfg.ResolveBaselinePath(home)

	existing, err := suppression.LoadBaseline(baselinePath)
	if err != nil {
		return "", fmt.Errorf("加载 baseline 失败: %w", err)
	}

	// 合并:union(保留已有 + 添加新发现)。无 baseline 时新建。
	var bs *suppression.BaselineSet
	added := 0
	if existing != nil {
		bs = existing
		if bs.Fingerprints == nil {
			bs.Fingerprints = make(map[string]bool)
		}
		for fp := range fps {
			if !bs.Fingerprints[fp] {
				bs.Fingerprints[fp] = true
				added++
			}
		}
	} else {
		bs = &suppression.BaselineSet{
			Version:      "1",
			Fingerprints: fps,
		}
		added = len(fps)
	}
	bs.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	if err := bs.Save(baselinePath); err != nil {
		return "", fmt.Errorf("保存 baseline 失败: %w", err)
	}
	return fmt.Sprintf("baseline 已生成(合并): %s\n  扫描产出 %d 条 finding, 新增 %d 条指纹, 合并后共 %d 条唯一指纹\n",
		baselinePath, len(res.Findings), added, len(bs.Fingerprints)), nil
}

// runBaselinePruneCmd 执行 --prune:加载旧 baseline → 重新扫描 → 删已不复现指纹 → 保存。
func runBaselinePruneCmd(cmd *cobra.Command, cfg *config.Config, home string) error {
	out, err := runBaselinePrune(cfg, home)
	fmt.Fprint(cmd.OutOrStdout(), out)
	return err
}

// runBaselinePrune 重新扫描,删除 baseline 中已不复现的指纹,保存。
// 返回可读输出。
func runBaselinePrune(cfg *config.Config, home string) (string, error) {
	baselinePath := cfg.ResolveBaselinePath(home)
	old, err := suppression.LoadBaseline(baselinePath)
	if err != nil {
		return "", fmt.Errorf("加载 baseline 失败: %w", err)
	}
	if old == nil {
		return "", fmt.Errorf("baseline 文件不存在: %s(请先 --create)", baselinePath)
	}
	res, err := runFullScan(cfg, home)
	if err != nil {
		return "", err
	}
	current := collectFingerprints(res)

	// 保留:旧 baseline 中仍在当前扫描里复现的指纹
	pruned := make(map[string]bool)
	dropped := 0
	for fp := range old.Fingerprints {
		if current[fp] {
			pruned[fp] = true
		} else {
			dropped++
		}
	}
	old.Fingerprints = pruned
	old.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	if err := old.Save(baselinePath); err != nil {
		return "", fmt.Errorf("保存 pruned baseline 失败: %w", err)
	}
	return fmt.Sprintf("baseline 已清理: %s\n  保留 %d 条指纹, 删除 %d 条已不复现\n",
		baselinePath, len(pruned), dropped), nil
}
