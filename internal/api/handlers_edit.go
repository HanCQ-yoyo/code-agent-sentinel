package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/editor"
)

type editRequestBody struct {
	NewContent string `json:"new_content"`
	BaseHash   string `json:"base_hash"`
}

// previewAsset 只读预览:diff + 危险项 + 乐观锁校验,不写盘。
func (s *Server) previewAsset(c *gin.Context) {
	if s.Editor == nil {
		c.JSON(http.StatusInternalServerError, errorBody("not_implemented", "editor not configured"))
		return
	}
	var body editRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
		return
	}
	pr, err := s.Editor.Preview(c.Request.Context(), editor.EditRequest{
		AssetID:    c.Param("id"),
		NewContent: body.NewContent,
		BaseHash:   body.BaseHash,
	})
	if err != nil {
		if errors.Is(err, editor.ErrNotFound) {
			c.JSON(http.StatusNotFound, errorBody("not_found", "asset not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, errorBody("preview_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, pr)
}
