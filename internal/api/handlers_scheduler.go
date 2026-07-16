package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
)

// schedulerResponse 是 GET /api/scheduler 的响应。
// Interval 为人类可读 duration 字符串(如 "1h0m0s");LastRun/NextRun 为 RFC3339 字符串,零值空串。
type schedulerResponse struct {
	Enabled  bool   `json:"enabled"`
	Interval string `json:"interval"`
	LastRun  string `json:"last_run"`
	NextRun  string `json:"next_run"`
}

func (s *Server) getScheduler(c *gin.Context) {
	st := s.Scheduler.Status()
	c.JSON(http.StatusOK, schedulerResponse{
		Enabled:  st.Enabled,
		Interval: st.Interval.String(),
		LastRun:  formatTime(st.LastRun),
		NextRun:  formatTime(st.NextRun),
	})
}

type putSchedulerBody struct {
	Enabled  *bool  `json:"enabled"`
	Interval string `json:"interval"`
}

func (s *Server) putScheduler(c *gin.Context) {
	var body putSchedulerBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
		return
	}
	interval, err := time.ParseDuration(body.Interval)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_interval", "interval 无法解析: "+body.Interval))
		return
	}
	enabled := body.Enabled != nil && *body.Enabled
	if s.ConfigPath != "" {
		s.Config.ScanEnabled = enabled
		s.Config.ScanInterval = body.Interval
		if err := config.Save(s.ConfigPath, s.Config); err != nil {
			c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
			return
		}
	}
	if s.Scheduler != nil {
		s.Scheduler.Reconfigure(enabled, interval)
	}
	s.getScheduler(c) // 返回最新状态
}

// formatTime 把 time.Time 格式化为 RFC3339;零值返回 ""。
// 供 scheduler 响应的 last_run/next_run 使用(零值=尚未运行/未调度)。
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
