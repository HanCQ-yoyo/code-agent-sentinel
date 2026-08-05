package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/configengine"
)

// getAgents 返回所有 agent + scan_enabled(从 ScheduleRepo 读取,默认 true)。
func (s *Server) getAgents(c *gin.Context) {
	type agentResp struct {
		configengine.Agent
		ScanEnabled bool `json:"scan_enabled"`
	}
	agents := make([]agentResp, 0, len(s.Agents))
	for _, a := range s.Agents {
		se := true // 默认
		if s.SchedRepo != nil {
			scs, _ := s.SchedRepo.List()
			for _, sc := range scs {
				if sc.AgentID == a.ID {
					se = sc.Enabled
					break
				}
			}
		}
		agents = append(agents, agentResp{Agent: a, ScanEnabled: se})
	}
	c.JSON(http.StatusOK, gin.H{"agents": agents, "current": s.SelectedAgentID})
}

// putAgentScanEnabled 改 per-agent 扫描开关,持久化到 SQLite ScheduleRepo。
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
	// 持久化到 ScheduleRepo(nil 则跳过,仅内存生效)
	if s.SchedRepo != nil {
		// scan_enabled 通过 schedule 的 enabled/interval 表达:开=有默认 interval,关=清空
		if body.ScanEnabled {
			_ = s.SchedRepo.Upsert(agentID, true, "30m")
		} else {
			_ = s.SchedRepo.Upsert(agentID, false, "0s")
		}
	}
	// 同步到 ScheduleManager
	s.applySchedules()
	c.JSON(http.StatusOK, gin.H{"agent_id": agentID, "scan_enabled": body.ScanEnabled})
}
