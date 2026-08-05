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
	lang := s.userPrefLanguage()
	interval, enabled := s.schedPrefs("claude-code")
	c.JSON(http.StatusOK, settingsResponse{
		Language:     lang,
		ScanInterval: interval,
		ScanEnabled:  enabled,
		ClaudeDir:    s.Config.ClaudeDir,
		Discovery:    s.Config.Discovery,
	})
}

// putSettings 更新运行期可改字段(language/scan_interval/scan_enabled)并持久化到 SQLite。
// claude_dir/discovery/home_dir 需重启生效,运行期忽略并在 warnings 中说明。
func (s *Server) putSettings(c *gin.Context) {
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
		return
	}
	var warnings []string
	if v, ok := raw["language"].(string); ok {
		if s.UserPrefs != nil {
			_ = s.UserPrefs.Set("language", v)
		}
	}
	if v, ok := raw["scan_interval"].(string); ok {
		if v != "" {
			if _, err := time.ParseDuration(v); err != nil {
				c.JSON(http.StatusBadRequest, errorBody("bad_interval", "scan_interval 无法解析: "+v))
				return
			}
		}
		if s.SchedRepo != nil {
			_, enabled := s.schedPrefs("claude-code")
			_ = s.SchedRepo.Upsert("claude-code", enabled, v)
		}
	}
	if v, ok := raw["scan_enabled"].(bool); ok {
		if s.SchedRepo != nil {
			interval, _ := s.schedPrefs("claude-code")
			if interval == "" {
				interval = "30m"
			}
			_ = s.SchedRepo.Upsert("claude-code", v, interval)
		}
		s.applyScanToggle()
	}
	for _, k := range []string{"claude_dir", "discovery", "home_dir"} {
		if _, ok := raw[k]; ok {
			warnings = append(warnings, k+" 需重启生效,不在运行期修改")
		}
	}
	resp := map[string]any{
		"language":      s.userPrefLanguage(),
		"scan_interval": "",
		"scan_enabled":  false,
		"claude_dir":    s.Config.ClaudeDir,
		"discovery":     s.Config.Discovery,
	}
	if interval, enabled := s.schedPrefs("claude-code"); true {
		resp["scan_interval"] = interval
		resp["scan_enabled"] = enabled
	}
	if len(warnings) > 0 {
		resp["warnings"] = warnings
	}
	c.JSON(http.StatusOK, resp)
}

// userPrefLanguage 从 UserPrefsStore 读 language 偏好(nil store → 空串)。
func (s *Server) userPrefLanguage() string {
	if s.UserPrefs == nil {
		return ""
	}
	v, _ := s.UserPrefs.Get("language")
	return v
}

// schedPrefs 从 ScheduleRepo 读取 agentID 的 interval/enabled(nil store → "0s", false)。
func (s *Server) schedPrefs(agentID string) (interval string, enabled bool) {
	if s.SchedRepo == nil {
		return "0s", false
	}
	scs, _ := s.SchedRepo.List()
	for _, sc := range scs {
		if sc.AgentID == agentID {
			return sc.Interval, sc.Enabled
		}
	}
	return "0s", false
}

// applyScanToggle 传播 ScheduleManager 暂停状态(nil store → 全开)。
func (s *Server) applyScanToggle() {
	if s.ScheduleManager == nil {
		return
	}
	_, enabled := s.schedPrefs("claude-code")
	s.ScheduleManager.SetPaused(!enabled)
}
