package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	var p string
	switch scope {
	case "project":
		p = c.Query("path")
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
	// 按 scope 过滤资产(global 含 plugin;project 仅含选中项目 p 的资产)。
	// project scope 必须额外按 source_path 前缀(<p>/)过滤:Discover 全 agent 发现
	// 后,所有项目的 ScopeProject 资产都在 inv.Assets 里,仅按 scope 过滤会把其他
	// 项目的资产(<projB>/.claude/...)也带进来,根外资产会被 BuildTree 挂为根级
	// synthetic 节点泄漏进选中项目树。前缀边界 <p>+Separator 防 /home/foo 匹配
	// /home/foobar。
	var assets []configengine.Asset
	for _, a := range inv.Assets {
		if scope == "global" && (a.Scope == configengine.ScopeGlobal || a.Scope == configengine.ScopePlugin) {
			assets = append(assets, a)
		} else if scope == "project" && a.Scope == configengine.ScopeProject && strings.HasPrefix(a.SourcePath, p+string(filepath.Separator)) {
			assets = append(assets, a)
		}
	}
	// project 根可能缺失:discoverProjects 允许项目仅有根级 .mcp.json 而无 .claude/ 子目录
	// (见 discover_project.go fileExists 分支),但 BuildTree 要求 root 存在,缺失时返回
	// os.ErrNotExist → 此处 500 tree_failed「file does not exist」,前端点该标签即报错。
	// 降级:root 不存在时用 BuildTreeFromAssets 只展示资产(无真实目录下钻),保证文件树
	// 仍可见该项目的资产而非白屏。global 根缺失属异常配置,仍走 BuildTree 报错。
	if scope == "project" {
		if _, statErr := os.Stat(root); statErr != nil {
			c.JSON(http.StatusOK, s.Engine.BuildTreeFromAssets(root, assets))
			return
		}
	}
	tree, err := s.Engine.BuildTree(root, assets)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("tree_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, tree)
}
