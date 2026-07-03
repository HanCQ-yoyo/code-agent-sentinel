package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/security"
)

func (s *Server) getFindings(c *gin.Context) {
	if s.lastResult == nil {
		c.JSON(http.StatusOK, []security.Finding{})
		return
	}
	sev := security.Severity(c.Query("severity"))
	asset := c.Query("asset")
	var out []security.Finding
	for _, f := range s.lastResult.Findings {
		if (sev == "" || f.Severity == sev) && (asset == "" || f.AssetID == asset) {
			out = append(out, f)
		}
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) getHealth(c *gin.Context) {
	if s.lastResult == nil || s.lastResult.HealthScore == nil {
		inv, _ := s.Engine.Discover()
		c.JSON(http.StatusOK, security.ComputeHealth(inv.Assets, nil))
		return
	}
	c.JSON(http.StatusOK, s.lastResult.HealthScore)
}
