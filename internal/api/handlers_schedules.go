package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
)

// schedulesResponse 是 GET /api/schedules 的响应。
type schedulesResponse struct {
	Schedules []scheduleStatusJSON `json:"schedules"`
}

// scheduleStatusJSON 是单个任务状态的 JSON 形态(Interval 人类可读 + 时间 RFC3339)。
type scheduleStatusJSON struct {
	AgentID  string `json:"agent_id"`
	Enabled  bool   `json:"enabled"`
	Interval string `json:"interval"`
	LastRun  string `json:"last_run"`
	NextRun  string `json:"next_run"`
}

func (s *Server) getSchedules(c *gin.Context) {
	out := []scheduleStatusJSON{}
	if s.ScheduleManager != nil {
		for _, st := range s.ScheduleManager.Status() {
			out = append(out, scheduleStatusJSON{
				AgentID:  st.AgentID,
				Enabled:  st.Enabled,
				Interval: st.Interval.String(),
				LastRun:  formatTime(st.LastRun),
				NextRun:  formatTime(st.NextRun),
			})
		}
	}
	c.JSON(http.StatusOK, schedulesResponse{Schedules: out})
}

type scheduleBody struct {
	AgentID  string `json:"agent_id"`
	Enabled  bool   `json:"enabled"`
	Interval string `json:"interval"`
}

// validateInterval 解析 interval,返回 duration;无效返回 (0, false)。
func validateInterval(v string) (time.Duration, bool) {
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, false
	}
	return d, true
}

func (s *Server) postSchedule(c *gin.Context) {
	var b scheduleBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
		return
	}
	if _, ok := validateInterval(b.Interval); !ok {
		c.JSON(http.StatusBadRequest, errorBody("bad_interval", "interval 无效: "+b.Interval))
		return
	}
	// 去重:同 agent_id 已有任务报 409
	for _, sc := range s.Config.Schedules {
		if sc.AgentID == b.AgentID {
			c.JSON(http.StatusConflict, errorBody("duplicate", "agent "+b.AgentID+" 已有定时任务"))
			return
		}
	}
	s.Config.Schedules = append(s.Config.Schedules, config.ScheduleCfg{
		AgentID: b.AgentID, Enabled: b.Enabled, Interval: b.Interval,
	})
	if err := s.persistSchedules(); err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
		return
	}
	s.applySchedules()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) putSchedule(c *gin.Context) {
	agentID := c.Param("agent_id")
	var b scheduleBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
		return
	}
	if _, ok := validateInterval(b.Interval); !ok {
		c.JSON(http.StatusBadRequest, errorBody("bad_interval", "interval 无效: "+b.Interval))
		return
	}
	found := false
	for i := range s.Config.Schedules {
		if s.Config.Schedules[i].AgentID == agentID {
			s.Config.Schedules[i].Enabled = b.Enabled
			s.Config.Schedules[i].Interval = b.Interval
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, errorBody("not_found", "agent "+agentID+" 无定时任务"))
		return
	}
	if err := s.persistSchedules(); err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
		return
	}
	s.applySchedules()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) deleteSchedule(c *gin.Context) {
	agentID := c.Param("agent_id")
	out := s.Config.Schedules[:0]
	found := false
	for _, sc := range s.Config.Schedules {
		if sc.AgentID == agentID {
			found = true
			continue
		}
		out = append(out, sc)
	}
	if !found {
		c.JSON(http.StatusNotFound, errorBody("not_found", "agent "+agentID+" 无定时任务"))
		return
	}
	s.Config.Schedules = out
	if err := s.persistSchedules(); err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
		return
	}
	s.applySchedules()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// persistSchedules 落盘当前 config(ConfigPath 空则跳过,与 settings/scheduler 一致)。
func (s *Server) persistSchedules() error {
	if s.ConfigPath == "" {
		return nil
	}
	return config.Save(s.ConfigPath, s.Config)
}

// applySchedules 把当前 config.Schedules 增量同步到 ScheduleManager(空则停全部)。
func (s *Server) applySchedules() {
	if s.ScheduleManager != nil {
		s.ScheduleManager.Apply(s.Config.Schedules)
	}
}
