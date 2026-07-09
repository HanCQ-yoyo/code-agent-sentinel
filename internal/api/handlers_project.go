package api

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/configengine"
)

func (s *Server) getProject(c *gin.Context) {
	projects, _ := s.Engine.ListProjects()
	// 过滤"伪项目":项目路径的 .claude 恰是全局配置根(agents[0].RootDir =
	// <home>/.claude)时,该项目与全局视图是同一份资产(settings.json / memory /
	// skills...),在项目 tab 里纯属重复(用户把 home 本身登记成了项目)。
	// 这样的项目 root 等于全局 RootDir → 列表/树与全局 tab 完全重叠,故剔除。
	globalRoot := ""
	if len(s.Agents) > 0 {
		globalRoot = filepath.Clean(s.Agents[0].RootDir)
	}
	var out []configengine.Project
	for _, p := range projects {
		if globalRoot != "" && filepath.Clean(filepath.Join(p.Path, ".claude")) == globalRoot {
			continue
		}
		out = append(out, p)
	}
	c.JSON(http.StatusOK, gin.H{"projects": out})
}
