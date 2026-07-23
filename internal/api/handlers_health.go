package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/security"
)

func (s *Server) getFindings(c *gin.Context) {
	agentIDs := s.resolveAgentIDs(c)
	for _, id := range agentIDs {
		if !s.agentExists(id) {
			c.JSON(http.StatusBadRequest, errorBody("unknown_agent", "未知 agent: "+id))
			return
		}
	}

	if shouldAggregate(c, agentIDs) {
		s.getFindingsAggregated(c, agentIDs)
		return
	}

	// 单 agent 路径:返回最近一次 global 扫描的 findings(按 severity/asset 过滤)。
	agentID := agentIDs[0]
	latest := s.latestScan(agentID)
	if latest == nil {
		c.JSON(http.StatusOK, []security.Finding{})
		return
	}
	sev := security.Severity(c.Query("severity"))
	asset := c.Query("asset")
	var out []security.Finding
	for _, f := range latest.Findings {
		if (sev == "" || f.Severity == sev) && (asset == "" || f.AssetID == asset) {
			out = append(out, f)
		}
	}
	c.JSON(http.StatusOK, out)
}

// getFindingsAggregated 聚合模式:拼接各 agent 最近 global 扫描的 findings。
// 每条 finding 已带 Finding.AgentID(Task 2 Runner 回填),无需在 API 层再注入。
// 应用与单 agent 路径相同的 severity/asset 过滤。无扫描的 agent 不贡献 finding。
// 响应:[]security.Finding(拼接后),无 finding 时返回空数组。
func (s *Server) getFindingsAggregated(c *gin.Context, agentIDs []string) {
	sev := security.Severity(c.Query("severity"))
	asset := c.Query("asset")
	latestScans, _ := s.History.LatestForAgents(agentIDs)
	out := []security.Finding{}
	for _, id := range agentIDs {
		rec, ok := latestScans[id]
		if !ok || rec == nil {
			continue
		}
		for _, f := range rec.Findings {
			if (sev == "" || f.Severity == sev) && (asset == "" || f.AssetID == asset) {
				out = append(out, f)
			}
		}
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) getHealth(c *gin.Context) {
	agentIDs := s.resolveAgentIDs(c)
	for _, id := range agentIDs {
		if !s.agentExists(id) {
			c.JSON(http.StatusBadRequest, errorBody("unknown_agent", "未知 agent: "+id))
			return
		}
	}

	if shouldAggregate(c, agentIDs) {
		s.getHealthAggregated(c, agentIDs)
		return
	}

	// 单 agent 路径:有扫描返回其 HealthScore;无扫描回退 ComputeHealth(assets, nil)。
	agentID := agentIDs[0]
	eng := s.Runner.EngineFor(agentID)
	latest := s.latestScan(agentID)
	if latest == nil || latest.HealthScore == nil {
		inv, _ := eng.Discover()
		c.JSON(http.StatusOK, security.ComputeHealth(inv.Assets, nil))
		return
	}
	c.JSON(http.StatusOK, latest.HealthScore)
}

// getHealthAggregated 聚合模式:返回 per-agent 健康分数组,不计算跨 agent 总分
// (健康分公式不跨 agent 聚合——每 agent 的资产/finding 独立评分)。
// 响应:gin.H{"is_aggregate": true, "agent_scores": []gin.H{
//   {"agent_id", "agent_name", "health_score": *HealthScore}}}
// 每 agent 的 health_score 取自其最近 global 扫描;无扫描时回退
// ComputeHealth(its assets, nil)(与单 agent no-scan 路径一致,无 finding → 100)。
func (s *Server) getHealthAggregated(c *gin.Context, agentIDs []string) {
	latestScans, _ := s.History.LatestForAgents(agentIDs)
	type agentScore struct {
		AgentID     string                `json:"agent_id"`
		AgentName   string                `json:"agent_name"`
		HealthScore *security.HealthScore `json:"health_score"`
	}
	scores := make([]agentScore, 0, len(agentIDs))
	for _, id := range agentIDs {
		hs := (*security.HealthScore)(nil)
		if rec, ok := latestScans[id]; ok && rec != nil && rec.HealthScore != nil {
			hs = rec.HealthScore
		} else {
			// 无扫描或无健康分:回退 ComputeHealth(assets, nil)。
			eng := s.Runner.EngineFor(id)
			if eng != nil {
				inv, _ := eng.Discover()
				hs = security.ComputeHealth(inv.Assets, nil)
			}
		}
		scores = append(scores, agentScore{
			AgentID:     id,
			AgentName:   s.agentName(id),
			HealthScore: hs,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"is_aggregate": true,
		"agent_scores": scores,
	})
}
