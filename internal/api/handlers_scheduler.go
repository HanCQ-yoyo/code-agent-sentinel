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
//
// Task 7 起 /api/scheduler 标记 deprecated:新前端用 /api/schedules,旧端点保留
// 转发到 schedules。响应构造:
//   - ScheduleManager 非 nil:取 Status() 中 AgentID==SelectedAgentID 的任务
//     (无则取第一个,再无则返回零值),Interval 用 config.Schedules 的原始字符串
//     (避免 time.Duration.String() 把 "30m" 漂移成 "30m0s")。
//   - Manager 为 nil:退化用 config.ScanEnabled/ScanInterval(向后兼容)。
//
// 不再走 s.Scheduler 单任务路径(旧端点 deprecated);但 putScheduler 仍同步调用
// s.Scheduler.Reconfigure(若非 nil),保留对老测试与未迁移代码的兼容。
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

// scheduleIntervalString 返回 agentID 对应任务的原始 interval 字符串(与 degrade 分支一致,
// 避免 "30m" 经 Duration.String() 漂移成 "30m0s")。找不到则回退 "0s"。
func (s *Server) scheduleIntervalString(agentID string) string {
	for _, sc := range s.Config.Schedules {
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
	// interval<=0 视为关闭(Task 7 语义:Start 对 interval<=0 no-op)。
	// 与 putSettings 行为一致:零/负 interval 强制 enabled=false 再 Reconfigure。
	if interval <= 0 {
		enabled = false
	}
	// 旧字段同步写(向后兼容读 ScanEnabled/ScanInterval 的代码)。
	s.Config.ScanEnabled = enabled
	s.Config.ScanInterval = body.Interval
	// Task 7:旧端点 deprecated 转发到 schedules——更新 SelectedAgentID 对应任务,
	// 不存在则追加。新前端应用 /api/schedules,这里仅为兼容旧调用方。
	agentID := s.SelectedAgentID
	if agentID == "" {
		agentID = "claude-code"
	}
	found := false
	for i := range s.Config.Schedules {
		if s.Config.Schedules[i].AgentID == agentID {
			s.Config.Schedules[i].Enabled = enabled
			s.Config.Schedules[i].Interval = body.Interval
			found = true
			break
		}
	}
	if !found {
		s.Config.Schedules = append(s.Config.Schedules, config.ScheduleCfg{
			AgentID: agentID, Enabled: enabled, Interval: body.Interval,
		})
	}
	if s.ConfigPath != "" {
		if err := config.Save(s.ConfigPath, s.Config); err != nil {
			c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
			return
		}
	}
	// Correction 1:保留对 s.Scheduler 单任务调度器的同步(若非 nil)。
	// TestPutSchedulerEnablesAndPersists 直接断言 s.Scheduler.Status().Enabled 翻转,
	// 丢掉此调用会让该测试失败。
	if s.Scheduler != nil {
		s.Scheduler.Reconfigure(enabled, interval)
	}
	// Task 7:同步到多任务 ScheduleManager(若非 nil),与新 /api/schedules 一致。
	s.applySchedules()
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
