// Package scan 提供 RunScan 抽象:发现→扫描→持久化历史的统一路径,
// 供 HTTP handler / scheduler / CLI 共用,避免逻辑漂移。
package scan

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/history"
	"code-agent-sentinel/internal/security"
)

// Runner 持有多 agent 的扫描依赖(Engine 按需懒构造并缓存)。
type Runner struct {
	agents       []configengine.Agent
	Orchestrator *security.Orchestrator
	History      *history.Store
	mu           sync.Mutex
	engines      map[string]*configengine.Engine // agentID → Engine,懒构造
}

// NewRunner 构造一个共享的扫描 Runner,持多个 agent。
func NewRunner(agents []configengine.Agent, orch *security.Orchestrator, hist *history.Store) *Runner {
	return &Runner{agents: agents, Orchestrator: orch, History: hist, engines: map[string]*configengine.Engine{}}
}

// EngineFor 返回 agentID 对应的 Engine(懒构造并缓存)。空 agentID 回退首 agent。
func (r *Runner) EngineFor(agentID string) *configengine.Engine {
	r.mu.Lock()
	defer r.mu.Unlock()
	if agentID == "" && len(r.agents) > 0 {
		agentID = r.agents[0].ID
	}
	if eng, ok := r.engines[agentID]; ok {
		return eng
	}
	// 找 agent 描述
	var a configengine.Agent
	for _, x := range r.agents {
		if x.ID == agentID {
			a = x
			break
		}
	}
	if a.ID == "" && len(r.agents) > 0 {
		a = r.agents[0] // 兜底
	}
	eng := configengine.NewEngineFromAgent(a)
	r.engines[agentID] = eng
	return eng
}

// RunScan 执行发现→扫描→持久化历史。agentID 空=回退首 agent;detectorIDs 空=全量检测器。
// 历史写入失败不阻断(降级体验,与原 postScan 一致)。
func (r *Runner) RunScan(ctx context.Context, agentID string, detectorIDs []string) (*security.ScanResult, error) {
	eng := r.EngineFor(agentID)
	inv, err := eng.Discover()
	if err != nil {
		return nil, fmt.Errorf("发现资产失败: %w", err)
	}
	res, err := r.Orchestrator.Scan(ctx, inv.Assets, detectorIDs)
	if err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
	}
	r.saveHistory(agentID, res, &inv)
	return res, nil
}

// saveHistory 把扫描结果落盘。ID = StartedAt 时间戳 + 8hex 随机后缀(防同秒冲突)。
func (r *Runner) saveHistory(agentID string, res *security.ScanResult, inv *configengine.Inventory) {
	if r.History == nil {
		return
	}
	b := make([]byte, 4)
	rand.Read(b)
	rec := history.ScanRecord{
		ID:          res.StartedAt.Format("2006-01-02-15-04-05") + "-" + hex.EncodeToString(b),
		AgentID:     agentID,
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
