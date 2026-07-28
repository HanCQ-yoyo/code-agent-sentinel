// internal/api/handlers_intercept.go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/intercept"
)

// getInterceptList 返回拦截记录列表(最新在前,由 Store.List 排序)。
// nil-safe:Server.Intercept 未注入时返回空数组(镜像 getHistoryList)。
// 可选筛选 ?outcome=deny / allow / warn / ask —— 纯字符串比较,无需 strings 包。
func (s *Server) getInterceptList(c *gin.Context) {
	if s.Intercept == nil {
		c.JSON(http.StatusOK, []any{})
		return
	}
	list, err := s.Intercept.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("intercept_list_failed", err.Error()))
		return
	}
	if outcome := c.Query("outcome"); outcome != "" {
		var filtered []intercept.InterceptRecord
		for _, r := range list {
			if r.Outcome == outcome {
				filtered = append(filtered, r)
			}
		}
		list = filtered
	}
	if list == nil {
		list = []intercept.InterceptRecord{}
	}
	c.JSON(http.StatusOK, list)
}

// getInterceptDetail 按 :id 取单条拦截记录。
func (s *Server) getInterceptDetail(c *gin.Context) {
	if s.Intercept == nil {
		c.JSON(http.StatusNotFound, errorBody("not_found", "intercept disabled"))
		return
	}
	rec, err := s.Intercept.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, errorBody("not_found", err.Error()))
		return
	}
	c.JSON(http.StatusOK, rec)
}

// deleteIntercept 按 :id 删除单条拦截记录。
func (s *Server) deleteIntercept(c *gin.Context) {
	if s.Intercept == nil {
		c.JSON(http.StatusNotFound, errorBody("not_found", "intercept disabled"))
		return
	}
	if err := s.Intercept.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, errorBody("not_found", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
