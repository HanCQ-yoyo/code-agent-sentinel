// Package scan 提供 RunScan 抽象:发现→扫描→持久化历史的统一路径,
// 供 HTTP handler / scheduler / CLI 共用,避免逻辑漂移。
package scan

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/history"
	"code-agent-sentinel/internal/security"
)

// ScanScope 描述一次扫描的范围。
type ScanScope struct {
	Type string // "global" | "project" | "asset"
	Path string // project: 项目路径;asset: 资产 source_path;global: 空
}

// scopeAssets 按 scope 过滤 inv.Assets。
// global/空 → 全量;project → ScopeProject 且 source_path 前缀匹配(与 getTree 一致);
// asset → source_path == scope.Path(扫同物理文件的所有 sibling);
// 未知 type 退化为全量。
func scopeAssets(inv configengine.Inventory, scope ScanScope) []configengine.Asset {
	switch scope.Type {
	case "", "global":
		return inv.Assets
	case "project":
		prefix := scope.Path + string(filepath.Separator)
		out := make([]configengine.Asset, 0)
		for _, a := range inv.Assets {
			if a.Scope == configengine.ScopeProject && strings.HasPrefix(a.SourcePath, prefix) {
				out = append(out, a)
			}
		}
		return out
	case "asset":
		out := make([]configengine.Asset, 0)
		for _, a := range inv.Assets {
			if a.SourcePath == scope.Path {
				out = append(out, a)
			}
		}
		return out
	}
	return inv.Assets // 未知 type 退化为全量
}

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
// scope 控制扫描范围(global/project/asset);历史写入失败不阻断(降级体验,与原 postScan 一致)。
//
// agentID 归一化:空串在 EngineFor 内部回退首 agent,saveHistory 也须记录归一化后的
// agent ID(而非空串),否则 latestScan(agentID) 在 partialRescan dedup 时按真实
// agent ID 查不到空串记录 → prior 空 → 误报新增(多 agent 场景的跨 agent 误报根因)。
func (r *Runner) RunScan(ctx context.Context, agentID string, scope ScanScope, detectorIDs []string) (*security.ScanResult, error) {
	eng := r.EngineFor(agentID)
	// 归一化 agentID 供 saveHistory 记录:与 EngineFor 的回退逻辑一致(空 → 首 agent ID)。
	recordAgentID := agentID
	if recordAgentID == "" && len(r.agents) > 0 {
		recordAgentID = r.agents[0].ID
	}
	inv, err := eng.Discover()
	if err != nil {
		return nil, fmt.Errorf("发现资产失败: %w", err)
	}
	assets := scopeAssets(inv, scope)
	res, err := r.Orchestrator.Scan(ctx, assets, detectorIDs)
	if err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
	}
	// 回填 Finding.AgentID:Orchestrator 不感知 agent,此处统一注入。
	for i := range res.Findings {
		res.Findings[i].AgentID = recordAgentID
	}
	r.saveHistory(recordAgentID, scope, res, &inv)
	return res, nil
}

// saveHistory 把扫描结果落盘。ID = StartedAt 时间戳 + 8hex 随机后缀(防同秒冲突)。
// scope 写入记录;空 scope.Type 归一化为 "global"。
func (r *Runner) saveHistory(agentID string, scope ScanScope, res *security.ScanResult, inv *configengine.Inventory) {
	if r.History == nil {
		return
	}
	b := make([]byte, 4)
	rand.Read(b)
	scopeType := scope.Type
	if scopeType == "" {
		scopeType = "global"
	}
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
		Scope:       scopeType,
		ScopePath:   scope.Path,
	}
	_ = r.History.Save(rec) // 持久化失败不阻断
}
