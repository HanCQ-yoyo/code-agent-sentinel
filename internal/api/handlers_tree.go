package api

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/configengine"
)

func (s *Server) getTree(c *gin.Context) {
	scope := c.Query("scope")
	inv, err := s.Engine.Discover()
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("discover_failed", err.Error()))
		return
	}
	var root string
	switch scope {
	case "project":
		p := c.Query("path")
		if p == "" {
			c.JSON(http.StatusBadRequest, errorBody("bad_request", "path required for project scope"))
			return
		}
		// 根校验:path 必须是 ListProjects() 已知项目之一,防越权遍历。
		known, _ := s.Engine.ListProjects()
		ok := false
		for _, pr := range known {
			if pr.Path == p {
				ok = true
				break
			}
		}
		if !ok {
			c.JSON(http.StatusBadRequest, errorBody("unknown_project", "path is not a known project"))
			return
		}
		root = filepath.Join(p, ".claude")
		if _, err := filepath.Abs(root); err != nil {
			c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
			return
		}
	default: // "global" 或缺省
		if len(s.Agents) == 0 {
			c.JSON(http.StatusInternalServerError, errorBody("no_agent", "no agent configured"))
			return
		}
		root = s.Agents[0].RootDir
		scope = "global"
	}
	// 按 scope 过滤资产(global 含 plugin;project 含该项目资产)。
	var assets []configengine.Asset
	for _, a := range inv.Assets {
		if scope == "global" && (a.Scope == configengine.ScopeGlobal || a.Scope == configengine.ScopePlugin) {
			assets = append(assets, a)
		} else if scope == "project" && a.Scope == configengine.ScopeProject {
			assets = append(assets, a)
		}
	}
	tree, err := s.Engine.BuildTree(root, assets)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("tree_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, tree)
}
