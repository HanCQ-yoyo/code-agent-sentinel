package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/security"
	"code-agent-sentinel/internal/security/findingstate"
	"code-agent-sentinel/internal/security/ruleengine"
)

// postFindingState 设置/更新单条处置状态。
// POST /api/finding-state  body: {fingerprint, status, priority?, note?}
func (s *Server) postFindingState(c *gin.Context) {
	var req struct {
		Fingerprint string `json:"fingerprint"`
		Status      string `json:"status"`
		Priority    string `json:"priority"`
		Note        string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
		return
	}
	if req.Fingerprint == "" || req.Status == "" {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", "fingerprint and status required"))
		return
	}
	st := s.FindingStates
	if st == nil {
		st = &findingstate.States{}
	}
	st.Set(req.Fingerprint, findingstate.State{
		Status:    findingstate.Status(req.Status),
		Priority:  req.Priority,
		Note:      req.Note,
		Source:    findingstate.SourceManual,
		UpdatedAt: nowUTC(),
	})
	// Set 已实时写 db,无需额外 Save
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// getFindingState 读单条处置状态。
// GET /api/finding-state/:fp
func (s *Server) getFindingState(c *gin.Context) {
	fp := c.Param("fp")
	st := s.FindingStates
	if st == nil {
		c.JSON(http.StatusNotFound, errorBody("not_found", "no states"))
		return
	}
	state, ok := st.Match(fp)
	if !ok {
		c.JSON(http.StatusNotFound, errorBody("not_found", "fingerprint not found"))
		return
	}
	c.JSON(http.StatusOK, state)
}

// deleteFindingState 重置/删除单条处置状态。
// DELETE /api/finding-state/:fp  (重置回 open = 删除记录)
func (s *Server) deleteFindingState(c *gin.Context) {
	fp := c.Param("fp")
	st := s.FindingStates
	if st == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	st.Remove(fp)
	// Remove 已实时写 db,无需额外 Save
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// postBulkAccept 批量接受。
// POST /api/finding-state/bulk-accept  body: {fingerprints: [...], source?}
func (s *Server) postBulkAccept(c *gin.Context) {
	var req struct {
		Fingerprints []string `json:"fingerprints"`
		Source       string   `json:"source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
		return
	}
	src := findingstate.SourceBulkAccept
	if req.Source != "" {
		src = findingstate.Source(req.Source)
	}
	st := s.FindingStates
	if st == nil {
		st = &findingstate.States{}
	}
	st.BulkAccept(req.Fingerprints, src, nowUTC())
	// BulkAccept 内部调 Set 已实时写 db,无需额外 Save
	c.JSON(http.StatusOK, gin.H{"ok": true, "count": len(req.Fingerprints)})
}

// getPruneReport 返回已处置但本轮未检出的孤儿状态。
// GET /api/finding-state/prune-report  query: active=fingerprint1,fingerprint2,...
func (s *Server) getPruneReport(c *gin.Context) {
	activeCSV := c.Query("active")
	var active []string
	if activeCSV != "" {
		active = splitCSV(activeCSV)
	}
	st := s.FindingStates
	if st == nil {
		c.JSON(http.StatusOK, gin.H{"orphans": []any{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"orphans": st.PruneReport(active)})
}

// attachCategory 为 findings 派生 Category 字段(API 读时填,不进 ScanRecord 持久化)。
// Category 由 rule_id 派生(ruleengine.CategoryOf),detector 不填 → 持久化时 omitempty 空
// → 反序列化空 → API 读时重新 attach。老记录也能读时派生。
func attachCategory(findings []security.Finding) []security.Finding {
	for i := range findings {
		findings[i].Category = string(ruleengine.CategoryOf(findings[i].RuleID))
	}
	return findings
}

// postBaseline 跑一次全量扫描,把所有非空 fingerprint 批量接受(accepted)到 finding_states。
// 语义变更(Task 11):旧实现 union 到 baseline.json(已删);新实现调 findingstate.BulkAccept,
// 与 POST /api/finding-state/bulk-accept 一致。agent 化:Discover 经选中 agent 的 Engine 跑。
func (s *Server) postBaseline(c *gin.Context) {
	eng, _, err := s.engineForQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorBody("unknown_agent", err.Error()))
		return
	}
	inv, err := eng.Discover()
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("discover_failed", err.Error()))
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()
	res, err := s.Orchestrator.Scan(ctx, inv.Assets, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("scan_failed", err.Error()))
		return
	}

	// 收集所有非空 fingerprint(仅 RulesDetector 的 finding 带 fingerprint)。
	seen := map[string]bool{}
	for _, f := range res.Findings {
		if f.Fingerprint != "" {
			seen[f.Fingerprint] = true
		}
	}
	fps := make([]string, 0, len(seen))
	for fp := range seen {
		fps = append(fps, fp)
	}

	st := s.FindingStates
	if st == nil {
		st = &findingstate.States{}
	}
	st.BulkAccept(fps, findingstate.SourceBulkAccept, nowUTC())
	// BulkAccept 内部调 Set 已实时写 db,无需额外 Save

	c.JSON(http.StatusOK, gin.H{
		"states_path":    "finding_states (sqlite)",
		"accepted_count": len(fps),
		"scan_findings":  len(res.Findings),
	})
}
