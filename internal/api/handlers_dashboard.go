package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) getDashboard(c *gin.Context) {
	eng, agentID, err := s.engineForQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorBody("unknown_agent", err.Error()))
		return
	}
	inv, err := eng.Discover()
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("discover_failed", err.Error()))
		return
	}
	counts := map[string]int{}
	for _, a := range inv.Assets {
		counts[string(a.Type)]++
	}
	dash := gin.H{
		"asset_counts": counts,
		"duplicates":   inv.Duplicates,
		"detectors":    s.detectorStatuses(),
		"last_scan":    s.latestScan(agentID),
		"agent":        agentID,
		"agent_name":   s.agentName(agentID),
	}
	c.JSON(http.StatusOK, dash)
}
