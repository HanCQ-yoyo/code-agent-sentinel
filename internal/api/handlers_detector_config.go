package api

import (
	"encoding/json"
	"fmt"
	"io"
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
//
// 安全校验(防部分体静默禁用):DetectorsConfig 用纯 bool 字段,零值=false=禁用,
// JSON 反序列化后无法区分"键缺失"与"显式 false"。故 PUT 须校验请求体含全部三个
// 顶层检测器键(rules/secret/dep)且 dep.engines 含 npm+govulncheck,否则部分体会
// 因 bool 零值静默禁用未指定的检测器(安全相关:丢失检测覆盖)。
func (s *Server) putDetectorConfig(c *gin.Context) {
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
	for _, key := range []string{"rules", "secret", "dep"} {
		if _, ok := top[key]; !ok {
			c.JSON(http.StatusBadRequest, errorBody("invalid_config",
				fmt.Sprintf("missing detector key %q; partial config silently disables unspecified detectors — send all of rules/secret/dep", key)))
			return
		}
	}
	var incoming config.DetectorsConfig
	if err := json.Unmarshal(raw, &incoming); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("invalid_config", err.Error()))
		return
	}
	// dep.engines 须含 npm + govulncheck(同因:防部分引擎配置静默禁用)。
	if _, ok := incoming.Dep.Engines["npm"]; !ok {
		c.JSON(http.StatusBadRequest, errorBody("invalid_config", `missing dep engine "npm"`))
		return
	}
	if _, ok := incoming.Dep.Engines["govulncheck"]; !ok {
		c.JSON(http.StatusBadRequest, errorBody("invalid_config", `missing dep engine "govulncheck"`))
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
// 返回指针而非值:DetectorsConfig 含 sync.RWMutex(go vet copylocks 会标记值拷贝),
// 交给 c.JSON(interface{}) 时按指针传递避免拷贝;快照是逐字段新建的,该 mutex 为零值未用。
func (s *Server) detectorConfig() *config.DetectorsConfig {
	s.Config.EnsureDetectors()
	// 读访问器已有锁;这里取一份快照供序列化。
	dc := s.Config.Detectors
	return &config.DetectorsConfig{
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
