package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/history"
)

func (s *Server) getDashboard(c *gin.Context) {
	agentIDs := s.resolveAgentIDs(c)
	// 校验每个 agent 合法性:未知 agent → 400 unknown_agent(不静默跳过)。
	for _, id := range agentIDs {
		if !s.agentExists(id) {
			c.JSON(http.StatusBadRequest, errorBody("unknown_agent", "未知 agent: "+id))
			return
		}
	}

	if shouldAggregate(c, agentIDs) {
		s.getDashboardAggregated(c, agentIDs)
		return
	}

	// 单 agent 路径:保持既有响应字段(agent/agent_name/last_scan/duplicates)。
	agentID := agentIDs[0]
	eng := s.Runner.EngineFor(agentID)
	inv, err := eng.Discover()
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("discover_failed", err.Error()))
		return
	}
	counts := map[string]int{}
	for _, a := range inv.Assets {
		counts[string(a.Type)]++
	}
	c.JSON(http.StatusOK, gin.H{
		"asset_counts": counts,
		"duplicates":   inv.Duplicates,
		"detectors":    s.detectorStatuses(),
		"last_scan":    s.latestScan(agentID),
		"agent":        agentID,
		"agent_name":   s.agentName(agentID),
	})
}

// getDashboardAggregated 聚合模式:循环各 agent 独立 Discover,汇总 asset_counts,
// 返回 agent_scans 数组(每项含 agent_id/agent_name/last_scan)。
// 不计算跨 agent 健康分(公式不跨 agent 聚合);每 agent 的健康分保留在其 last_scan 中。
// 单 agent Discover 失败不中断聚合(跳过该 agent 的资产计数,但仍保留其 agent_scans 条目)。
func (s *Server) getDashboardAggregated(c *gin.Context, agentIDs []string) {
	latestScans, _ := s.History.LatestForAgents(agentIDs)
	totalAssets := map[string]int{}
	type agentScanInfo struct {
		AgentID   string             `json:"agent_id"`
		AgentName string             `json:"agent_name"`
		LastScan  *history.ScanRecord `json:"last_scan,omitempty"`
	}
	agentScans := make([]agentScanInfo, 0, len(agentIDs))
	for _, id := range agentIDs {
		eng := s.Runner.EngineFor(id)
		if eng != nil {
			if inv, err := eng.Discover(); err == nil {
				for _, a := range inv.Assets {
					totalAssets[string(a.Type)]++
				}
			}
		}
		info := agentScanInfo{AgentID: id, AgentName: s.agentName(id)}
		if rec, ok := latestScans[id]; ok {
			info.LastScan = rec
		}
		agentScans = append(agentScans, info)
	}
	c.JSON(http.StatusOK, gin.H{
		"asset_counts": totalAssets,
		"detectors":    s.detectorStatuses(),
		"agent_scans":  agentScans,
		"is_aggregate": true,
	})
}
