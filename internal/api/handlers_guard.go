// internal/api/handlers_guard.go
package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
)

// guardConfigSnapshot 返回 GuardConfig 的快照(逐字段新建,避免拷贝 mutex)。
// 读访问器均 nil-safe:即使 c.Guard 为 nil 也能返回全启用默认值。
func (s *Server) guardConfigSnapshot() *config.GuardConfig {
	s.Config.EnsureGuard()
	g := s.Config.Guard
	return &config.GuardConfig{
		Enabled:          g.EnabledEffective(),
		Policy:           g.PolicyOrDefault(),
		DeadlineMS:       g.DeadlineOrDefault(),
		MaxCommandBytes:  g.MaxBytesOrDefault(),
		Mode:             g.ModeOrDefault(),
		AllowlistEnabled: g.AllowlistEnabledOrDefault(),
	}
}

func (s *Server) getGuardConfig(c *gin.Context) {
	c.JSON(http.StatusOK, s.guardConfigSnapshot())
}

// putGuardConfig 校验并持久化 guard 配置:ApplyFrom 原地改写,再 config.Save 回写文件。
//
// 安全校验(防部分体静默禁用):GuardConfig 用 bool/int/string 字段,零值=false=禁用 / 0=默认
// / 空串=默认,JSON 反序列化后无法区分”键缺失”与”显式 false/0/空串”。故 PUT 须校验请求体含全部
// 六个顶层键(enabled/policy/deadline_ms/max_command_bytes/mode/allowlist_enabled),否则部分体
// 会因零值静默禁用 guard(发送 {enabled:false} 会同时把 deadline_ms 置 0 → 200 默认,但
// policy=空串 → “deny”;全键检查强制客户端发送完整配置,与 putDetectorConfig 同思路)。
// Stage R3:加 mode/allowlist_enabled(二者零值=空串 → strict / false → 关闭放行,均需显式发送)。
func (s *Server) putGuardConfig(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorBody("invalid_config", err.Error()))
		return
	}
	// 两遍解码:先 map 检查顶层键齐全(防部分体静默禁用),再结构体。
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("invalid_config", err.Error()))
		return
	}
	for _, key := range []string{"enabled", "policy", "deadline_ms", "max_command_bytes", "mode", "allowlist_enabled"} {
		if _, ok := top[key]; !ok {
			c.JSON(http.StatusBadRequest, errorBody("invalid_config",
				"missing guard key "+key+"; partial config silently keeps defaults — send all of enabled/policy/deadline_ms/max_command_bytes/mode/allowlist_enabled"))
			return
		}
	}
	var incoming config.GuardConfig
	if err := json.Unmarshal(raw, &incoming); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("invalid_config", err.Error()))
		return
	}
	s.Config.EnsureGuard()
	s.Config.Guard.ApplyFrom(&incoming)
	if err := config.Save(s.ConfigPath, s.Config); err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, s.guardConfigSnapshot())
}
