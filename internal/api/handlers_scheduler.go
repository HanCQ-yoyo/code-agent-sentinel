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

// schedulerStatusResponse 构造 scheduler 响应,nil-safe。
// 优先用 s.Scheduler.Status()(实时调度状态);否则从 s.Config 退化构造:
//   - Enabled 退化为 s.Config.ScanEnabled
//   - Interval 退化为 s.Config.ScanInterval 字符串(空则 "0s",与 duration 零值一致)
//   - LastRun/NextRun 退化空串(无运行记录)
//
// 这保证 s.Scheduler == nil 时(main.go 未注入、测试、或未启动调度)
// GET / PUT /api/scheduler 仍返回 200 + 基于 config 的状态,不 panic。
func (s *Server) schedulerStatusResponse() schedulerResponse {
	if s.Scheduler != nil {
		st := s.Scheduler.Status()
		return schedulerResponse{
			Enabled:  st.Enabled,
			Interval: st.Interval.String(),
			LastRun:  formatTime(st.LastRun),
			NextRun:  formatTime(st.NextRun),
		}
	}
	// 退化:基于 config 构造。ScanInterval 空串时用 "0s"(duration 零值的 String())。
	interval := s.Config.ScanInterval
	if interval == "" {
		interval = "0s"
	}
	return schedulerResponse{
		Enabled:  s.Config.ScanEnabled,
		Interval: interval,
		LastRun:  "",
		NextRun:  "",
	}
}

func (s *Server) getScheduler(c *gin.Context) {
	c.JSON(http.StatusOK, s.schedulerStatusResponse())
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
	// interval<=0 视为关闭(Task 7 语义:Start 对 interval<=0 no-op)。
	// 与 putSettings 行为一致:零/负 interval 强制 enabled=false 再 Reconfigure。
	if interval <= 0 {
		enabled = false
	}
	// 始终更新内存 config(与 putSettings 一致);仅当 ConfigPath 非空时落盘。
	s.Config.ScanEnabled = enabled
	s.Config.ScanInterval = body.Interval
	if s.ConfigPath != "" {
		if err := config.Save(s.ConfigPath, s.Config); err != nil {
			c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
			return
		}
	}
	if s.Scheduler != nil {
		s.Scheduler.Reconfigure(enabled, interval)
	}
	c.JSON(http.StatusOK, s.schedulerStatusResponse()) // 返回最新状态(nil-safe)
}

// formatTime 把 time.Time 格式化为 RFC3339;零值返回 ""。
// 供 scheduler 响应的 last_run/next_run 使用(零值=尚未运行/未调度)。
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
