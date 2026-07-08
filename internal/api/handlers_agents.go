package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) getAgents(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"agents": s.Agents, "current": s.SelectedAgentID})
}
