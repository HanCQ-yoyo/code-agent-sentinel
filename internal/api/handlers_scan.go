package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/history"
)

func (s *Server) postScan(c *gin.Context) {
	var ids []string
	if d := c.Query("detectors"); d != "" {
		ids = strings.Split(d, ",")
	}
	agentID := c.Query("agent")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()
	res, err := s.Runner.RunScan(ctx, agentID, ids)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("scan_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, res)
}

func (s *Server) getScanResult(c *gin.Context) {
	latest := s.latestScan()
	if latest == nil {
		c.JSON(http.StatusOK, struct{}{})
		return
	}
	c.JSON(http.StatusOK, latest)
}

// latestScan 返回最近一次扫描的完整记录;无历史或 History 未配置返回 nil。
func (s *Server) latestScan() *history.ScanRecord {
	if s.History == nil {
		return nil
	}
	latest, err := s.History.Latest()
	if err != nil || latest == nil {
		return nil
	}
	return latest
}
