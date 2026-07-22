package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/editor"
	"code-agent-sentinel/internal/scan"
	"code-agent-sentinel/internal/security"
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

// commitAsset 备份+原子写+部分重扫,返回更新资产 + new_findings。
func (s *Server) commitAsset(c *gin.Context) {
	if s.Editor == nil {
		c.JSON(http.StatusInternalServerError, errorBody("not_implemented", "editor not configured"))
		return
	}
	var body editRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
		return
	}
	res, err := s.Editor.Commit(c.Request.Context(), editor.EditRequest{
		AssetID:    c.Param("id"),
		NewContent: body.NewContent,
		BaseHash:   body.BaseHash,
	})
	if err != nil {
		switch {
		case errors.Is(err, editor.ErrNotFound):
			c.JSON(http.StatusNotFound, errorBody("not_found", "asset not found"))
		case errors.Is(err, editor.ErrNotEditable):
			c.JSON(http.StatusForbidden, errorBody("not_editable", err.Error()))
		case errors.Is(err, editor.ErrOutOfRoot):
			c.JSON(http.StatusForbidden, errorBody("out_of_root", err.Error()))
		case errors.Is(err, editor.ErrConcurrentModification):
			c.JSON(http.StatusConflict, errorBody("concurrent_modification", err.Error()))
		case errors.Is(err, editor.ErrBadContent):
			c.JSON(http.StatusBadRequest, errorBody("bad_content", err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, errorBody("write_failed", err.Error()))
		}
		return
	}
	// 部分重扫:受影响资产(同 source_path)+ rules 检测器。
	// 经 Runner.RunScan 走 agent 抽象(agentID 取 ?agent= 或 SelectedAgentID),
	// 不再硬编码 s.Engine/Orchestrator(Task 13:统一扫描路径)。
	newFindings, rescanErr := s.partialRescan(s.agentIDForRequest(c), res.Asset)
	resp := gin.H{
		"asset":        res.Asset,
		"backup_path":  res.BackupPath,
		"diff":         res.Diff,
		"dangerous":    res.Dangerous,
		"new_findings": newFindings,
	}
	if rescanErr != "" {
		resp["rescan_error"] = rescanErr
	}
	c.JSON(http.StatusOK, resp)
}

// partialRescan 跑受影响资产(同 source_path)的 rules 检测器,对比 latest 同检测器 findings 返回新增。
// 返回 (newFindings, rescanError):rescanError 非空时表示重扫失败(Discover/Scan 错误或 res==nil),
// 前端据此提示用户手动全量重扫,而非误报"无新增风险"。
//
// Task 13:改造为经 s.Runner.RunScan(ScanScope{Type:"asset"})走 agent 抽象,不再硬编码
// s.Engine/Orchestrator。agentID 由 commitAsset 从 ?agent= 或 SelectedAgentID 传入;
// 未知 agentID 经 Runner.EngineFor 兜底回退首 agent(容错,与 engineForQuery 的 400 语义不同:
// 编辑已落盘,rescan 失败可降级,不应因 agent 拼写错让整个 commit 响应报错)。
//
// 行为变化:RunScan 内部 saveHistory 会写一条 asset-scope 历史记录(旧 partialRescan 直接调
// Orchestrator.Scan 不写历史)。LatestForAgent 优先 global scope,故 asset-scope 记录不污染
// dashboard 的 latest-global;仅作为审计轨迹留存(谁在何时对哪个资产做了 rescan)。
//
// prior 须在 RunScan 之前取:RunScan 会 saveHistory 写一条 asset-scope 记录,若 prior 在其后取,
// latestScan("") 在无 global 历史的全新场景下可能退化为取该 asset-scope 记录 → prior=fresh →
// 全部被去重 → new_findings 恒空(回归)。prior 取"本次 rescan 之前"的 latest,语义正确。
func (s *Server) partialRescan(agentID string, updated configengine.Asset) (fresh []security.Finding, rescanError string) {
	// rescan 不应 panic 崩溃整个 commit 响应(写入已成功);recover 兜底。
	defer func() {
		if r := recover(); r != nil {
			fresh = make([]security.Finding, 0)
			rescanError = fmt.Sprintf("partial rescan failed: panic %v", r)
		}
	}()
	// 先取 prior(本次 rescan 之前的 latest 同 source_path sibling findings),
	// 必须在 RunScan 之前:RunScan 会 saveHistory,若之后取可能读到刚写的 asset-scope 记录。
	prior := s.priorFindingsForSourcePath(updated.SourcePath, []string{"rules"})
	priorKeys := map[string]bool{}
	for _, f := range prior {
		priorKeys[findingKey(f)] = true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// RunScan 内部:EngineFor(agentID) → Discover → scopeAssets(asset, updated.SourcePath)
	//   → 扫同 source_path 的全部 sibling(settings + permissions + per-hook)→ saveHistory。
	// scopeAssets 的 asset 分支与旧 affected 过滤同为 a.SourcePath == path,行为一致。
	res, err := s.Runner.RunScan(ctx, agentID, scan.ScanScope{Type: "asset", Path: updated.SourcePath}, []string{"rules"})
	if err != nil || res == nil {
		msg := "partial rescan failed: scan returned nil"
		if err != nil {
			msg = "partial rescan failed: " + err.Error()
		}
		return make([]security.Finding, 0), msg
	}
	fresh = make([]security.Finding, 0)
	for _, f := range res.Findings {
		if !priorKeys[findingKey(f)] {
			fresh = append(fresh, f)
		}
	}
	return fresh, ""
}

// priorFindingsForSourcePath 从 latest scan 取同 source_path 的所有 sibling 资产
// + 指定检测器的 findings。同 source_path 的资产(settings + permissions + per-hook)
// 共享一个物理文件,重扫覆盖全部 sibling,但 rules findings 的 AssetID
// 是被扫 sibling 的 ID(如 permissions.ID),故须用全部 sibling AssetID 过滤 prior。
func (s *Server) priorFindingsForSourcePath(sourcePath string, detectorIDs []string) []security.Finding {
	latest := s.latestScan("")
	if latest == nil {
		return nil
	}
	// 收集同 source_path 的全部 AssetID。
	siblingIDs := map[string]bool{}
	if latest.Inventory != nil {
		for _, a := range latest.Inventory.Assets {
			if a.SourcePath == sourcePath {
				siblingIDs[a.ID] = true
			}
		}
	}
	// 若 latest 无 inventory(旧历史记录),回退:从 latest findings 的 AssetID 无法
	// 推断 source_path,此时 prior 为空(可能少量误报新增,但不影响安全性)。
	allow := map[string]bool{}
	for _, d := range detectorIDs {
		allow[d] = true
	}
	out := make([]security.Finding, 0)
	for _, f := range latest.Findings {
		if siblingIDs[f.AssetID] && allow[f.DetectorID] {
			out = append(out, f)
		}
	}
	return out
}

// findingKey 生成 finding 去重键。Finding.ID 是 P1 遗留死字段(检测器从未设值),
// 故用 (DetectorID, RuleID, AssetID, Evidence) 复合键:同一规则在同一资产上
// 触发且 evidence 不变视为同一条;编辑使 evidence 变化(如 Bash(git:*)→Bash(*))
// 则 key 变 → 报为新增,符合部分重扫「标出新危险」语义。
func findingKey(f security.Finding) string {
	return f.DetectorID + "\x00" + f.RuleID + "\x00" + f.AssetID + "\x00" + f.Evidence
}
