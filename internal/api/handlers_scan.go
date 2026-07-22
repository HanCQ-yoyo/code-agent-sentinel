package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/history"
	"code-agent-sentinel/internal/scan"
)

func (s *Server) postScan(c *gin.Context) {
	var ids []string
	if d := c.Query("detectors"); d != "" {
		ids = strings.Split(d, ",")
	}
	agentID := c.Query("agent")
	// 未知 agent(非空且不在 s.Agents)→ 400 unknown_agent。
	// 空串仍回退首 agent(EngineFor 既有契约,兼容 sentinel scan 无 --agent、scheduler 内部调用)。
	if agentID != "" && !s.agentExists(agentID) {
		c.JSON(http.StatusBadRequest, errorBody("unknown_agent", "未知 agent: "+agentID))
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()
	// Task 11:scope 占位传 global(Task 14 将从 ?scope=/?path= 构造真实 scope)。
	res, err := s.Runner.RunScan(ctx, agentID, scan.ScanScope{Type: "global"}, ids)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("scan_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, res)
}

func (s *Server) getScanResult(c *gin.Context) {
	agentID := c.Query("agent")
	latest := s.latestScan(agentID)
	if latest == nil {
		c.JSON(http.StatusOK, struct{}{})
		return
	}
	c.JSON(http.StatusOK, latest)
}

// latestScan 返回指定 agent 最近一次扫描的完整记录;空 agentID 退化为全局最新。
// 无历史或 History 未配置返回 nil。
func (s *Server) latestScan(agentID string) *history.ScanRecord {
	if s.History == nil {
		return nil
	}
	latest, err := s.History.LatestForAgent(agentID)
	if err != nil || latest == nil {
		return nil
	}
	return latest
}
