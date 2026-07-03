package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/configengine"
)

func (s *Server) getAssets(c *gin.Context) {
	inv, err := s.Engine.Discover()
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("discover_failed", err.Error()))
		return
	}
	typ := configengine.AssetType(c.Query("type"))
	scope := configengine.Scope(c.Query("scope"))
	if typ != "" || scope != "" {
		inv.Assets = inv.Filter(typ, scope)
	}
	c.JSON(http.StatusOK, inv)
}

func (s *Server) getAsset(c *gin.Context) {
	inv, err := s.Engine.Discover()
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("discover_failed", err.Error()))
		return
	}
	id := c.Param("id")
	for _, a := range inv.Assets {
		if a.ID == id {
			c.JSON(http.StatusOK, a)
			return
		}
	}
	c.JSON(http.StatusNotFound, errorBody("not_found", "asset not found"))
}
