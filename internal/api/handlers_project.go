package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) getProject(c *gin.Context) {
	projects, _ := s.Engine.ListProjects()
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}
