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
	// 部分重扫:受影响资产(同 source_path)+ baseline/injection 检测器。
	newFindings, rescanErr := s.partialRescan(res.Asset)
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

// partialRescan 跑受影响资产(同 source_path)的 baseline+injection,对比 latest 同检测器 findings 返回新增。
// 返回 (newFindings, rescanError):rescanError 非空时表示重扫失败(Discover/Scan 错误或 fresh=nil),
// 前端据此提示用户手动全量重扫,而非误报"无新增风险"。
func (s *Server) partialRescan(updated configengine.Asset) (fresh []security.Finding, rescanError string) {
	// rescan 不应 panic 崩溃整个 commit 响应(写入已成功);recover 兜底。
	defer func() {
		if r := recover(); r != nil {
			fresh = make([]security.Finding, 0)
			rescanError = fmt.Sprintf("partial rescan failed: panic %v", r)
		}
	}()
	inv, err := s.Engine.Discover()
	if err != nil {
		return make([]security.Finding, 0), "partial rescan failed: " + err.Error()
	}
	var affected []configengine.Asset
	for _, a := range inv.Assets {
		if a.SourcePath == updated.SourcePath {
			affected = append(affected, a)
		}
	}
	if len(affected) == 0 {
		return make([]security.Finding, 0), "partial rescan failed: no affected assets found"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := s.Orchestrator.Scan(ctx, affected, []string{"baseline", "content.injection"})
	if err != nil || res == nil {
		msg := "partial rescan failed: scan returned nil"
		if err != nil {
			msg = "partial rescan failed: " + err.Error()
		}
		return make([]security.Finding, 0), msg
	}
	// 该资产在 latest 同检测器的 findings 集合(对比基线)。
	// 同 source_path 的所有 sibling 资产(settings + permissions + per-hook)都参与重扫,
	// baseline/injection findings 的 AssetID 是被扫 sibling 的 ID(如 permissions.ID),
	// 而非编辑资产的 ID,故 prior 须收集 ALL sibling AssetID。
	prior := s.priorFindingsForSourcePath(updated.SourcePath, []string{"baseline", "content.injection"})
	priorKeys := map[string]bool{}
	for _, f := range prior {
		priorKeys[findingKey(f)] = true
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
// 共享一个物理文件,重扫覆盖全部 sibling,但 baseline/injection findings 的 AssetID
// 是被扫 sibling 的 ID(如 permissions.ID),故须用全部 sibling AssetID 过滤 prior。
func (s *Server) priorFindingsForSourcePath(sourcePath string, detectorIDs []string) []security.Finding {
	latest := s.latestScan()
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

// priorFindingsForAsset 从 latest scan 取该资产 + 指定检测器的 findings。
func (s *Server) priorFindingsForAsset(assetID string, detectorIDs []string) []security.Finding {
	latest := s.latestScan()
	if latest == nil {
		return nil
	}
	allow := map[string]bool{}
	for _, d := range detectorIDs {
		allow[d] = true
	}
	var out []security.Finding
	for _, f := range latest.Findings {
		if f.AssetID == assetID && allow[f.DetectorID] {
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
