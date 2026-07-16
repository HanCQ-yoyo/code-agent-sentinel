package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
)

// pinnedResponse 是 GET/PUT /api/pinned-projects 的响应体:置顶项目列表。
// 空列表返回 [] 而非 null(前端可直接 .length / 遍历)。
type pinnedResponse struct {
	PinnedProjects []config.PinnedProject `json:"pinned_projects"`
}

// getPinnedProjects 返回当前置顶的项目列表。
func (s *Server) getPinnedProjects(c *gin.Context) {
	c.JSON(http.StatusOK, pinnedResponse{PinnedProjects: s.pinnedList()})
}

// pinnedList 返回置顶项目切片(空则为 []config.PinnedProject{},非 nil),
// 并过滤 path 为空的损坏条目。
func (s *Server) pinnedList() []config.PinnedProject {
	if s.Config.PinnedProjects == nil {
		return []config.PinnedProject{}
	}
	out := make([]config.PinnedProject, 0, len(s.Config.PinnedProjects))
	for _, p := range s.Config.PinnedProjects {
		if p.Path != "" {
			out = append(out, p)
		}
	}
	return out
}

// putPinnedBody 是 PUT /api/pinned-projects 的请求体:完整置顶列表(非增量)。
type putPinnedBody struct {
	PinnedProjects []config.PinnedProject `json:"pinned_projects"`
}

// putPinnedProjects 用请求体整体替换置顶列表并持久化到配置文件。
//
// 整体替换(非增量)语义与 favorites / dir-tags 一致:前端持有完整列表,
// 增删后整体回写。校验:颜色限定预设 6 色(防任意值),path 为空的条目跳过。
// 持久化到 ~/.claude-sentinel/config.yaml。
func (s *Server) putPinnedProjects(c *gin.Context) {
	var body putPinnedBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
		return
	}
	// 校验颜色在预设 6 色内(防任意值)
	validColors := map[string]bool{"red": true, "orange": true, "gold": true, "green": true, "blue": true, "purple": true}
	clean := make([]config.PinnedProject, 0, len(body.PinnedProjects))
	for _, p := range body.PinnedProjects {
		if p.Path == "" {
			continue
		}
		if p.Color != "" && !validColors[p.Color] {
			c.JSON(http.StatusBadRequest, errorBody("bad_color", "未知颜色: "+p.Color))
			return
		}
		clean = append(clean, p)
	}
	s.Config.PinnedProjects = clean
	if s.ConfigPath != "" {
		if err := config.Save(s.ConfigPath, s.Config); err != nil {
			c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
			return
		}
	}
	c.JSON(http.StatusOK, pinnedResponse{PinnedProjects: s.pinnedList()})
}
