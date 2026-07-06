package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/history"
)

func (s *Server) getHistoryList(c *gin.Context) {
	if s.History == nil {
		c.JSON(http.StatusOK, []any{})
		return
	}
	list, err := s.History.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("history_list_failed", err.Error()))
		return
	}
	if list == nil {
		list = []history.ScanSummary{}
	}
	c.JSON(http.StatusOK, list)
}

func (s *Server) getHistoryDetail(c *gin.Context) {
	if s.History == nil {
		c.JSON(http.StatusNotFound, errorBody("not_found", "history disabled"))
		return
	}
	rec, err := s.History.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, errorBody("not_found", err.Error()))
		return
	}
	c.JSON(http.StatusOK, rec)
}

func (s *Server) deleteHistory(c *gin.Context) {
	if s.History == nil {
		c.JSON(http.StatusNotFound, errorBody("not_found", "history disabled"))
		return
	}
	if err := s.History.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, errorBody("not_found", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
