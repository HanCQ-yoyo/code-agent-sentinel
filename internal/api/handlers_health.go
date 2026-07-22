package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/security"
)

func (s *Server) getFindings(c *gin.Context) {
	latest := s.latestScan("")
	if latest == nil {
		c.JSON(http.StatusOK, []security.Finding{})
		return
	}
	sev := security.Severity(c.Query("severity"))
	asset := c.Query("asset")
	var out []security.Finding
	for _, f := range latest.Findings {
		if (sev == "" || f.Severity == sev) && (asset == "" || f.AssetID == asset) {
			out = append(out, f)
		}
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) getHealth(c *gin.Context) {
	latest := s.latestScan("")
	if latest == nil || latest.HealthScore == nil {
		inv, _ := s.Engine.Discover()
		c.JSON(http.StatusOK, security.ComputeHealth(inv.Assets, nil))
		return
	}
	c.JSON(http.StatusOK, latest.HealthScore)
}
