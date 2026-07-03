package api

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/configengine"
)

func (s *Server) getProject(c *gin.Context) {
	projects, _ := s.Engine.ListProjects()
	resp := gin.H{"current": s.Engine.Project, "projects": projects}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) postProject(c *gin.Context) {
	p := c.Query("path")
	if p == "" {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", "path required"))
		return
	}
	s.Engine.SelectProject(configengine.Project{Path: p, Name: filepath.Base(p)})
	c.JSON(http.StatusOK, gin.H{"current": s.Engine.Project})
}
