package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
)

// pinnedResponse 是 GET/PUT /api/pinned-projects 的响应体:置顶项目列表。
type pinnedResponse struct {
	PinnedProjects []config.PinnedProject `json:"pinned_projects"`
}

// getPinnedProjects 返回当前置顶的项目列表(从 SQLite UserPrefsStore 读取)。
func (s *Server) getPinnedProjects(c *gin.Context) {
	c.JSON(http.StatusOK, pinnedResponse{PinnedProjects: s.pinnedList()})
}

// pinnedList 返回置顶项目切片(空则为 []config.PinnedProject{},非 nil),
// 并过滤 path 为空的损坏条目。
func (s *Server) pinnedList() []config.PinnedProject {
	if s.UserPrefs == nil {
		return []config.PinnedProject{}
	}
	v, err := s.UserPrefs.Get("pinned_projects")
	if err != nil || v == "" {
		return []config.PinnedProject{}
	}
	var projs []config.PinnedProject
	if err := json.Unmarshal([]byte(v), &projs); err != nil {
		return []config.PinnedProject{}
	}
	out := make([]config.PinnedProject, 0, len(projs))
	for _, p := range projs {
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

// putPinnedProjects 用请求体整体替换置顶列表并持久化到 SQLite UserPrefsStore。
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
	if s.UserPrefs != nil {
		b, _ := json.Marshal(clean)
		_ = s.UserPrefs.Set("pinned_projects", string(b))
	}
	c.JSON(http.StatusOK, pinnedResponse{PinnedProjects: clean})
}
