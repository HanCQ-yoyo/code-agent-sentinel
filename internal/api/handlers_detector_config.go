package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
)

// getDetectorConfig 返回当前检测器运行期配置(展开默认,便于前端表单回显)。
func (s *Server) getDetectorConfig(c *gin.Context) {
	cfg := s.detectorConfig()
	c.JSON(http.StatusOK, cfg)
}

// putDetectorConfig 校验并持久化检测器配置:ApplyFrom 原地改写运行期 cfg.Detectors
// (检测器持指针即时生效),再 config.Save 回写文件。
func (s *Server) putDetectorConfig(c *gin.Context) {
	var incoming config.DetectorsConfig
	if err := c.ShouldBindJSON(&incoming); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("invalid_config", err.Error()))
		return
	}
	s.Config.EnsureDetectors()
	s.Config.Detectors.ApplyFrom(&incoming)
	if err := config.Save(s.ConfigPath, s.Config); err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, s.detectorConfig())
}

// detectorConfig 返回 cfg.Detectors 的快照(含默认展开),nil 时返回全启用默认。
func (s *Server) detectorConfig() config.DetectorsConfig {
	s.Config.EnsureDetectors()
	// 读访问器已有锁;这里取一份快照供序列化。
	dc := s.Config.Detectors
	return config.DetectorsConfig{
		Rules:  config.DetectorToggle{Enabled: dc.RulesEnabled()},
		Secret: config.BinaryDetectorConfig{Enabled: dc.SecretEnabled(), Binary: dc.SecretBinaryOrDefault()},
		Dep: config.DepDetectorConfig{
			Enabled: dc.DepEnabled(),
			Engines: map[string]config.BinaryDetectorConfig{
				"npm":         {Enabled: dc.DepEngineEnabled("npm"), Binary: dc.DepEngineBinaryOrDefault("npm")},
				"govulncheck": {Enabled: dc.DepEngineEnabled("govulncheck"), Binary: dc.DepEngineBinaryOrDefault("govulncheck")},
			},
		},
	}
}
