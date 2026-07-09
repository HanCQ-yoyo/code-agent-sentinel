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

// getDirTags 返回默认目录标签 + 用户已保存的覆盖。
// Overrides 可能为 nil(用户未自定义)→ JSON null,前端按空 map 处理。
func (s *Server) getDirTags(c *gin.Context) {
	ov := s.Config.DirTags
	if ov == nil {
		ov = config.DirTags{}
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

// putDirTags 用请求体整体替换用户覆盖映射并持久化到配置文件。
//
// 整体替换(非增量合并)语义简单:前端持有当前完整 overrides,编辑后整体回写,
// 删除某标签 = 从 map 移除后回写。校验:仅允许已知标签值(config/runtime),
// 防 payload 写入任意字符串污染配置。
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
		// key 非空即可;路径合法性由前端保证(相对 .claude 根的路径段)。
		if k == "" {
			c.JSON(http.StatusBadRequest, errorBody("bad_key", "empty tag key"))
			return
		}
	}
	s.Config.DirTags = body.Overrides
	if s.ConfigPath == "" {
		// 测试场景无路径:仅内存更新,不持久化。
		c.JSON(http.StatusOK, dirTagsResponse{Defaults: config.DefaultDirTags(), Overrides: body.Overrides})
		return
	}
	if err := config.Save(s.ConfigPath, s.Config); err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, dirTagsResponse{Defaults: config.DefaultDirTags(), Overrides: body.Overrides})
}
