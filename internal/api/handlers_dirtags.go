package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
)

// dirTagsResponse 返回默认标签 + 用户覆盖。前端合并后用 ResolveDirTag 逻辑算生效标签。
type dirTagsResponse struct {
	Defaults  config.DirTags `json:"defaults"`
	Overrides config.DirTags `json:"overrides"`
}

// getDirTags 返回默认目录标签 + 用户已保存的覆盖(从 SQLite UserPrefsStore 读取)。
func (s *Server) getDirTags(c *gin.Context) {
	ov := config.DirTags{}
	if s.UserPrefs != nil {
		v, _ := s.UserPrefs.Get("dir_tags")
		if v != "" {
			// JSON 反序列化;失败返回空 map(nil store → 空)
		jsonUnmarshalDirTags(v, &ov)
		}
	}
	c.JSON(http.StatusOK, dirTagsResponse{
		Defaults:  config.DefaultDirTags(),
		Overrides: ov,
	})
}

// putDirTagsBody 是 PUT /api/dir-tags 的请求体:完整覆盖映射(非增量)。
type putDirTagsBody struct {
	Overrides config.DirTags `json:"overrides"`
}

// putDirTags 用请求体整体替换用户覆盖映射并持久化到 SQLite UserPrefsStore。
func (s *Server) putDirTags(c *gin.Context) {
	var body putDirTagsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
		return
	}
	for k, v := range body.Overrides {
		if v != config.TagConfig && v != config.TagRuntime {
			c.JSON(http.StatusBadRequest, errorBody("bad_tag", "unknown tag value: "+v))
			return
		}
		if k == "" {
			c.JSON(http.StatusBadRequest, errorBody("bad_key", "empty tag key"))
			return
		}
	}
	// 持久化到 UserPrefsStore(nil store 静默跳过)
	if s.UserPrefs != nil {
		_ = s.UserPrefs.Set("dir_tags", jsonMarshalDirTags(body.Overrides))
	}
	c.JSON(http.StatusOK, dirTagsResponse{Defaults: config.DefaultDirTags(), Overrides: body.Overrides})
}
