package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
)

type settingsResponse struct {
	Language     string               `json:"language"`
	ScanInterval string               `json:"scan_interval"`
	ScanEnabled  bool                 `json:"scan_enabled"`
	ClaudeDir    string               `json:"claude_dir"`
	Discovery    *config.DiscoveryCfg `json:"discovery"`
}

func (s *Server) getSettings(c *gin.Context) {
	c.JSON(http.StatusOK, settingsResponse{
		Language:     s.Config.Language,
		ScanInterval: s.Config.ScanInterval,
		ScanEnabled:  s.Config.ScanEnabled,
		ClaudeDir:    s.Config.ClaudeDir,
		Discovery:    s.Config.Discovery,
	})
}

// putSettings 更新运行期可改字段(language/scan_interval/scan_enabled)并落盘。
// claude_dir/discovery/home_dir 需重启生效,运行期忽略并在 warnings 中说明。
// body 用 raw map 读取一次(c.ShouldBindJSON 只能读一次 body):
// 先读原始 map,再按类型断言逐字段取值。
func (s *Server) putSettings(c *gin.Context) {
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
		return
	}
	var warnings []string
	scanChanged := false
	if v, ok := raw["language"].(string); ok {
		s.Config.Language = v
	}
	if v, ok := raw["scan_interval"].(string); ok {
		if v != "" {
			if _, err := time.ParseDuration(v); err != nil {
				c.JSON(http.StatusBadRequest, errorBody("bad_interval", "scan_interval 无法解析: "+v))
				return
			}
		}
		s.Config.ScanInterval = v
		scanChanged = true
	}
	if v, ok := raw["scan_enabled"].(bool); ok {
		s.Config.ScanEnabled = v
		scanChanged = true
	}
	if s.ConfigPath != "" {
		if err := config.Save(s.ConfigPath, s.Config); err != nil {
			c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
			return
		}
	}
	if scanChanged {
		s.applyScanToggle()
	}
	for _, k := range []string{"claude_dir", "discovery", "home_dir"} {
		if _, ok := raw[k]; ok {
			warnings = append(warnings, k+" 需重启生效,不在运行期修改")
		}
	}
	resp := map[string]any{
		"language":      s.Config.Language,
		"scan_interval": s.Config.ScanInterval,
		"scan_enabled":  s.Config.ScanEnabled,
		"claude_dir":    s.Config.ClaudeDir,
		"discovery":     s.Config.Discovery,
	}
	if len(warnings) > 0 {
		resp["warnings"] = warnings
	}
	c.JSON(http.StatusOK, resp)
}

// applyScanToggle 把 scan_enabled 总开关状态传播到 ScheduleManager.Paused。
// 总开关关 → Paused=true,所有 per-agent 定时任务 tick 跳过(Status 仍报各自 enabled/interval);
// 总开关开 → Paused=false,各任务按自身 schedule.enabled 跑。
// scan_interval 不在此处强行覆盖 per-agent schedule.interval(后者以 /api/schedules 为准,
// scan_interval 仅作无 schedule 时的回退默认,见 ResolveSchedules)。
// 注:本方法不触已 dead 的 s.Scheduler.Reconfigure(后者由 Task 3 连同字段一并删除)。
func (s *Server) applyScanToggle() {
	if s.ScheduleManager == nil {
		return
	}
	s.ScheduleManager.SetPaused(!s.Config.ScanEnabled)
}
