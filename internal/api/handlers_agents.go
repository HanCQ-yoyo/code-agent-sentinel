package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
)

// getAgents 返回所有 agent + scan_enabled(默认 true)。
func (s *Server) getAgents(c *gin.Context) {
	type agentResp struct {
		configengine.Agent
		ScanEnabled bool `json:"scan_enabled"`
	}
	agents := make([]agentResp, 0, len(s.Agents))
	for _, a := range s.Agents {
		se := true // 默认
		// 从 config 查对应 AgentCfg 取 ScanEnabled
		for _, ac := range s.Config.Agents {
			if ac.ID == a.ID {
				if ac.ScanEnabled != nil {
					se = *ac.ScanEnabled
				}
				break
			}
		}
		agents = append(agents, agentResp{Agent: a, ScanEnabled: se})
	}
	c.JSON(http.StatusOK, gin.H{"agents": agents, "current": s.SelectedAgentID})
}

// putAgentScanEnabled 改 per-agent 扫描开关,写 config 持久化。
func (s *Server) putAgentScanEnabled(c *gin.Context) {
	agentID := c.Param("agent_id")
	if !s.agentExists(agentID) {
		c.JSON(http.StatusBadRequest, errorBody("unknown_agent", "未知 agent: "+agentID))
		return
	}
	var body struct {
		ScanEnabled bool `json:"scan_enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
		return
	}
	// 更新 s.Config.Agents 对应项
	for i := range s.Config.Agents {
		if s.Config.Agents[i].ID == agentID {
			s.Config.Agents[i].ScanEnabled = &body.ScanEnabled
			break
		}
	}
	// 持久化
	if err := config.Save(s.ConfigPath, s.Config); err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"agent_id": agentID, "scan_enabled": body.ScanEnabled})
}
