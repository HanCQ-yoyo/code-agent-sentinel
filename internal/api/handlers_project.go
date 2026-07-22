package api

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/configengine"
)

func (s *Server) getProject(c *gin.Context) {
	eng, _, err := s.engineForQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorBody("unknown_agent", err.Error()))
		return
	}
	projects, _ := eng.ListProjects()
	// 过滤"伪项目":项目路径的 .claude 恰是全局配置根(选中 agent 的 eng.ClaudeDir
	// = <home>/.claude)时,该项目与全局视图是同一份资产(settings.json / memory /
	// skills...),在项目 tab 里纯属重复(用户把 home 本身登记成了项目)。
	// 这样的项目 root 等于全局根 → 列表/树与全局 tab 完全重叠,故剔除。
	//
	// 用 eng.ClaudeDir 而非 s.Agents[0].RootDir:?agent=b 时 eng 是 b 的 Engine,
	// eng.ListProjects() 返回 b 的项目(读 home/.claude-b.json);若仍用
	// s.Agents[0].RootDir(agent a 的根)做过滤,b 若有项目登记在自己的根,会因
	// 与 a 的根不等而漏过滤;反之 a 的根也不该用来过滤 b 的项目列表。根必须匹配
	// 选中 agent(同 Task 7 getTree 的修复)。
	globalRoot := filepath.Clean(eng.ClaudeDir)
	var out []configengine.Project
	for _, p := range projects {
		if globalRoot != "" && filepath.Clean(filepath.Join(p.Path, ".claude")) == globalRoot {
			continue
		}
		out = append(out, p)
	}
	c.JSON(http.StatusOK, gin.H{"projects": out})
}
