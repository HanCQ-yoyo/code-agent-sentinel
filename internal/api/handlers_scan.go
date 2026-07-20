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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()
	// Task 4 临时:postScan 还未从请求读 agentID,先传空回退首 agent。Task 9 接 /api/scan?agent=...
	res, err := s.Runner.RunScan(ctx, "", ids)
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
