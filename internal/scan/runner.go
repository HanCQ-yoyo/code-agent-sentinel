// Package scan 提供 RunScan 抽象:发现→扫描→持久化历史的统一路径,
// 供 HTTP handler / scheduler / CLI 共用,避免逻辑漂移。
package scan

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/history"
	"code-agent-sentinel/internal/security"
)

// Runner 持有一次扫描所需的依赖(无状态,可被 HTTP/scheduler/CLI 共享)。
type Runner struct {
	Engine       *configengine.Engine
	Orchestrator *security.Orchestrator
	History      *history.Store
}

// NewRunner 构造一个共享的扫描 Runner。
func NewRunner(eng *configengine.Engine, orch *security.Orchestrator, hist *history.Store) *Runner {
	return &Runner{Engine: eng, Orchestrator: orch, History: hist}
}

// RunScan 执行发现→扫描→持久化历史。detectorIDs 空 = 全量检测器。
// 历史写入失败不阻断(降级体验,与原 postScan 一致)。
func (r *Runner) RunScan(ctx context.Context, detectorIDs []string) (*security.ScanResult, error) {
	inv, err := r.Engine.Discover()
	if err != nil {
		return nil, fmt.Errorf("发现资产失败: %w", err)
	}
	res, err := r.Orchestrator.Scan(ctx, inv.Assets, detectorIDs)
	if err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
	}
	r.saveHistory(res, &inv)
	return res, nil
}

// saveHistory 把扫描结果落盘。ID = StartedAt 时间戳 + 8hex 随机后缀(防同秒冲突)。
func (r *Runner) saveHistory(res *security.ScanResult, inv *configengine.Inventory) {
	if r.History == nil {
		return
	}
	b := make([]byte, 4)
	rand.Read(b)
	rec := history.ScanRecord{
		ID:          res.StartedAt.Format("2006-01-02-15-04-05") + "-" + hex.EncodeToString(b),
		StartedAt:   res.StartedAt,
		Duration:    res.Duration,
		Findings:    res.Findings,
		Detectors:   res.Detectors,
		HealthScore: res.HealthScore,
		Inventory:   inv,
		Projects:    inv.Projects,
	}
	_ = r.History.Save(rec) // 持久化失败不阻断
}
