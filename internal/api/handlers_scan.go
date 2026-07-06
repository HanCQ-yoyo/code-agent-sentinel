package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/history"
	"code-agent-sentinel/internal/security"
)

func (s *Server) postScan(c *gin.Context) {
	inv, err := s.Engine.Discover()
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("discover_failed", err.Error()))
		return
	}
	var ids []string
	if d := c.Query("detectors"); d != "" {
		ids = strings.Split(d, ",")
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()
	res, err := s.Orchestrator.Scan(ctx, inv.Assets, ids)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("scan_failed", err.Error()))
		return
	}
	// 持久化到历史目录(失败不阻断响应:扫描结果仍返回,历史缺失仅降级体验)。
	s.saveHistory(res, &inv)
	c.JSON(http.StatusOK, res)
}

// saveHistory 把扫描结果落盘。ID 用 StartedAt 时间戳 + 8hex 随机后缀(防同秒冲突)。
func (s *Server) saveHistory(res *security.ScanResult, inv *configengine.Inventory) {
	if s.History == nil {
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
		Project:     inv.Project,
	}
	_ = s.History.Save(rec) // 持久化失败不阻断 API
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
