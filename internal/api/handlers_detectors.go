package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) getDetectors(c *gin.Context) {
	c.JSON(http.StatusOK, s.detectorStatuses())
}

func (s *Server) detectorStatuses() []gin.H {
	var out []gin.H
	for _, d := range s.Orchestrator.Registry.Detectors() {
		out = append(out, gin.H{"id": d.ID(), "available": d.Available(), "reason": d.Reason()})
	}
	return out
}
