package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/security"
)

func (s *Server) getDetectors(c *gin.Context) {
	c.JSON(http.StatusOK, s.detectorStatuses())
}

func (s *Server) detectorStatuses() []security.DetectorMeta {
	var out []security.DetectorMeta
	for _, d := range s.Orchestrator.Registry.Detectors() {
		out = append(out, d.Meta())
	}
	return out
}
