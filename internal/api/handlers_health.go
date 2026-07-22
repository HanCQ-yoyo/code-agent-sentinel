package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/security"
)

func (s *Server) getFindings(c *gin.Context) {
	// getFindings 不直接调 Discover,但需 engineForQuery 来(a)校验/解析 ?agent=,
	// (b)拿 agentID 传给 latestScan。eng 本身不在此 handler 使用(只读 latest.Findings),
	// 故丢弃 eng 用 _。
	_, agentID, err := s.engineForQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorBody("unknown_agent", err.Error()))
		return
	}
	latest := s.latestScan(agentID)
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
	eng, agentID, err := s.engineForQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorBody("unknown_agent", err.Error()))
		return
	}
	latest := s.latestScan(agentID)
	if latest == nil || latest.HealthScore == nil {
		inv, _ := eng.Discover()
		c.JSON(http.StatusOK, security.ComputeHealth(inv.Assets, nil))
		return
	}
	c.JSON(http.StatusOK, latest.HealthScore)
}
