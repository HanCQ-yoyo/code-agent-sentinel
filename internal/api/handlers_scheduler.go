package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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
//
// Task 7 起 /api/scheduler 标记 deprecated:新前端用 /api/schedules,旧端点保留
// 转发到 schedules。ScheduleManager 非 nil 时取 Status() 中对应 agent 的任务。
// Manager 为 nil 时退化使用 ScheduleRepo(nil store → 零值)。
func (s *Server) schedulerStatusResponse() schedulerResponse {
	if s.ScheduleManager != nil {
		st := s.ScheduleManager.Status()
		agentID := s.SelectedAgentID
		if agentID == "" {
			agentID = "claude-code"
		}
		for _, x := range st {
			if x.AgentID == agentID {
				return schedulerResponse{
					Enabled:  x.Enabled,
					Interval: s.scheduleIntervalString(agentID),
					LastRun:  formatTime(x.LastRun),
					NextRun:  formatTime(x.NextRun),
				}
			}
		}
		if len(st) > 0 {
			x := st[0]
			return schedulerResponse{
				Enabled:  x.Enabled,
				Interval: s.scheduleIntervalString(x.AgentID),
				LastRun:  formatTime(x.LastRun),
				NextRun:  formatTime(x.NextRun),
			}
		}
		// 无任务
		return schedulerResponse{Enabled: false, Interval: "0s"}
	}
	// 退化:基于 ScheduleRepo(nil → 零值)
	interval, enabled := s.schedPrefs(s.SelectedAgentID)
	if interval == "" {
		interval = "0s"
	}
	return schedulerResponse{
		Enabled:  enabled,
		Interval: interval,
		LastRun:  "",
		NextRun:  "",
	}
}

// scheduleIntervalString 返回 agentID 对应任务的原始 interval 字符串。
// 从 ScheduleRepo 查找，找不到则回退 "0s"。
func (s *Server) scheduleIntervalString(agentID string) string {
	if s.SchedRepo == nil {
		return "0s"
	}
	scs, _ := s.SchedRepo.List()
	for _, sc := range scs {
		if sc.AgentID == agentID {
			return sc.Interval
		}
	}
	return "0s"
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
	// interval<=0 视为关闭
	if interval <= 0 {
		enabled = false
	}
	// 持久化到 ScheduleRepo
	agentID := s.SelectedAgentID
	if agentID == "" {
		agentID = "claude-code"
	}
	if s.SchedRepo != nil {
		_ = s.SchedRepo.Upsert(agentID, enabled, body.Interval)
	}
	// 同步到多任务 ScheduleManager
	s.applySchedules()
	c.JSON(http.StatusOK, s.schedulerStatusResponse())
}

// formatTime 把 time.Time 格式化为 RFC3339;零值返回 ""。
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
