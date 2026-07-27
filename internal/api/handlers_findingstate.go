package api

import (
	"context"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/security"
	"code-agent-sentinel/internal/security/findingstate"
	"code-agent-sentinel/internal/security/ruleengine"
)

// findingStatePath 返回 finding_states.yaml 路径(在 home/.claude-sentinel/)。
// 用 s.Engine.HomeDir(与 postBaseline 取路径方式一致),
// 不用 s.Config.Resolve* —— finding_states.yaml 暂不接 config 覆盖(统一默认路径)。
func (s *Server) findingStatePath() string {
	return filepath.Join(s.Engine.HomeDir, ".claude-sentinel", "finding_states.yaml")
}

// statesMu 保护 finding_states.yaml 的并发读写(多请求同时处置)。
var statesMu sync.Mutex

// loadStates 读取 finding_states.yaml;文件不存在或解析失败返回 nil(nil 安全)。
func (s *Server) loadStates() *findingstate.States {
	st, _ := findingstate.Load(s.findingStatePath())
	return st
}

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
	statesMu.Lock()
	defer statesMu.Unlock()
	st := s.loadStates()
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
	if err := st.Save(s.findingStatePath()); err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// getFindingState 读单条处置状态。
// GET /api/finding-state/:fp
func (s *Server) getFindingState(c *gin.Context) {
	fp := c.Param("fp")
	st := s.loadStates()
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
	statesMu.Lock()
	defer statesMu.Unlock()
	st := s.loadStates()
	if st == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	st.Remove(fp)
	if err := st.Save(s.findingStatePath()); err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
		return
	}
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
	statesMu.Lock()
	defer statesMu.Unlock()
	st := s.loadStates()
	if st == nil {
		st = &findingstate.States{}
	}
	st.BulkAccept(req.Fingerprints, src, nowUTC())
	if err := st.Save(s.findingStatePath()); err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
		return
	}
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
	st := s.loadStates()
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

// postBaseline 跑一次全量扫描,把所有非空 fingerprint 批量接受(accepted)到 finding_states.yaml。
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

	statesMu.Lock()
	defer statesMu.Unlock()
	st := s.loadStates()
	if st == nil {
		st = &findingstate.States{}
	}
	st.BulkAccept(fps, findingstate.SourceBulkAccept, nowUTC())
	if err := st.Save(s.findingStatePath()); err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"states_path":    s.findingStatePath(),
		"accepted_count": len(fps),
		"scan_findings":  len(res.Findings),
	})
}
