package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) getDashboard(c *gin.Context) {
	inv, err := s.Engine.Discover()
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
		"last_scan":    s.lastResult,
	}
	c.JSON(http.StatusOK, dash)
}
