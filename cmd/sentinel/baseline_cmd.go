package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security"
	"code-agent-sentinel/internal/security/findingstate"
	"code-agent-sentinel/internal/storage"
)

// newBaselineCmd 构造 `code-agent-sentinel baseline` 子命令(--create / --prune)。
//
// Task 11 语义变更:旧实现写 baseline.json(已删);新实现统一到 finding_states 表(SQLite):
//   - --create:跑全量扫描,把所有 Finding 的 fingerprint 批量接受(accepted),与 API POST /api/baseline 一致。
//   - --prune:重新扫描,删掉 finding_states 表中已不复现的孤儿状态(PruneReport + Remove)。
//
// 持久化:sqlite sentinel.db,路径 <home>/.code-agent-sentinel/sentinel.db。
// 扫描逻辑镜像 main.go run():构建 Engine + Registry + Orchestrator,跑全量 Scan。
func newBaselineCmd() *cobra.Command {
	var (
		cfgPath string
		create  bool
		prune   bool
	)
	cmd := &cobra.Command{
		Use:   "baseline",
		Short: "Manage baselines: --create bulk accept / --prune clean orphan states",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !create && !prune {
				return fmt.Errorf("specify --create or --prune")
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
	cmd.Flags().StringVar(&cfgPath, "config", "", "Config file path (default ~/.code-agent-sentinel/config.yaml)")
	cmd.Flags().BoolVar(&create, "create", false, "Run full scan and bulk accept fingerprints into finding_states (sqlite)")
	cmd.Flags().BoolVar(&prune, "prune", false, "Re-scan and remove orphan states no longer present in finding_states")
	return cmd
}

// runFullScan 镜像 main.go run() 的扫描设置,跑一次全量扫描返回 findings。
// 不启动 HTTP server。用传入的 cfg 解析路径(供未来 detector 读 cfg)。
// db 为规则库 sqlite 句柄,注入 RulesDetector(替代 Task 8 的临时 nil)。
func runFullScan(cfg *config.Config, home string, db *storage.DB) (*security.ScanResult, error) {
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
		return nil, fmt.Errorf("asset discovery failed: %w", err)
	}
	cfg.EnsureDetectors() // 与 main.go 一致:检测器持 cfg.Detectors 指针
	r := security.NewRegistry()
	r.Register(security.NewRulesDetector(home, cfg.Detectors, db))
	r.Register(security.NewSecretDetector(cfg.Detectors))
	r.Register(security.NewDependencyDetector(cfg.Detectors))
	orch := &security.Orchestrator{Registry: r}
	res, err := orch.Scan(context.Background(), inv.Assets, nil)
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
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

// runBaselineCreateCmd 执行 --create:全量扫描 → 批量接受 fingerprint → 持久化到 sqlite。
func runBaselineCreateCmd(cmd *cobra.Command, cfg *config.Config, home string) error {
	out, err := runBaselineCreate(cfg, home)
	fmt.Fprint(cmd.OutOrStdout(), out)
	return err
}

// runBaselineCreate 跑全量扫描,把全部 Finding 的 fingerprint 批量接受到 finding_states 表(sqlite)。
// 已有非 open 状态(resolved/false_positive/accepted/in_progress)不覆盖,尊重既有处置。
// 返回可读输出。
func runBaselineCreate(cfg *config.Config, home string) (string, error) {
	db, err := openBaselineDB(home)
	if err != nil {
		return "", err
	}
	defer db.Close()

	res, err := runFullScan(cfg, home, db)
	if err != nil {
		return "", err
	}
	fps := collectFingerprints(res)

	st := findingstate.NewStates(db)
	st.BulkAccept(fps, findingstate.SourceBulkAccept, time.Now().UTC().Format(time.RFC3339))
	// BulkAccept 内部调 Set 已实时写 db,无需额外 Save

	return fmt.Sprintf("Disposition states bulk accepted: finding_states (sqlite)\n  Scanned %d findings, bulk accepted %d fingerprints\n",
		len(res.Findings), len(fps)), nil
}

// runBaselinePruneCmd 执行 --prune:加载 finding_states → 重新扫描 → 删已不复现指纹 → 保存。
func runBaselinePruneCmd(cmd *cobra.Command, cfg *config.Config, home string) error {
	out, err := runBaselinePrune(cfg, home)
	fmt.Fprint(cmd.OutOrStdout(), out)
	return err
}

// runBaselinePrune 重新扫描,删除 finding_states 表中已不复现的指纹。
// 返回可读输出。
func runBaselinePrune(cfg *config.Config, home string) (string, error) {
	db, err := openBaselineDB(home)
	if err != nil {
		return "", err
	}
	defer db.Close()

	st := findingstate.NewStates(db)
	if len(st.Items) == 0 {
		return "", fmt.Errorf("finding_states table is empty (run --create first)")
	}
	res, err := runFullScan(cfg, home, db)
	if err != nil {
		return "", err
	}
	current := collectFingerprints(res)

	// 用 PruneReport 找出孤儿(已处置但本轮未检出),逐条 Remove。
	orphans := st.PruneReport(current)
	for _, o := range orphans {
		st.Remove(o.Fingerprint)
	}
	// Remove 已实时写 db,无需额外 Save

	remain := len(st.Items)
	return fmt.Sprintf("Disposition states cleaned: finding_states (sqlite)\n  Kept %d, removed %d no longer present\n",
		remain, len(orphans)), nil
}

// openBaselineDB 打开 sqlite 规则库,建表,同步 builtin 规则(用于 baseline 子命令)。
// 路径: <home>/.code-agent-sentinel/sentinel.db(与 main.go 一致)。
func openBaselineDB(home string) (*storage.DB, error) {
	dbPath := filepath.Join(home, ".code-agent-sentinel", "sentinel.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	db, err := storage.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open rule DB: %w", err)
	}
	initialized, initErr := storage.SchemaInitialized(db)
	if initErr != nil {
		db.Close()
		return nil, fmt.Errorf("check rule DB schema: %w", initErr)
	}
	if !initialized {
		if err := storage.RunMigrations(db); err != nil {
			db.Close()
			return nil, fmt.Errorf("run migrations: %w", err)
		}
	}
	syncBuiltinRules(db)
	return db, nil
}
