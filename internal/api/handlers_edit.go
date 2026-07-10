package api

import (
	"context"
	"errors"
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
	newFindings := s.partialRescan(res.Asset)
	c.JSON(http.StatusOK, gin.H{
		"asset":        res.Asset,
		"backup_path":  res.BackupPath,
		"diff":         res.Diff,
		"dangerous":    res.Dangerous,
		"new_findings": newFindings,
	})
}

// partialRescan 跑受影响资产(同 source_path)的 baseline+injection,对比 latest 同检测器 findings 返回新增。
func (s *Server) partialRescan(updated configengine.Asset) []security.Finding {
	inv, err := s.Engine.Discover()
	if err != nil {
		return nil
	}
	var affected []configengine.Asset
	for _, a := range inv.Assets {
		if a.SourcePath == updated.SourcePath {
			affected = append(affected, a)
		}
	}
	if len(affected) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := s.Orchestrator.Scan(ctx, affected, []string{"baseline", "content.injection"})
	if err != nil || res == nil {
		return nil
	}
	// 该资产在 latest 同检测器的 findings 集合(对比基线)
	prior := s.priorFindingsForAsset(updated.ID, []string{"baseline", "content.injection"})
	priorKeys := map[string]bool{}
	for _, f := range prior {
		priorKeys[findingKey(f)] = true
	}
	var fresh []security.Finding
	for _, f := range res.Findings {
		if !priorKeys[findingKey(f)] {
			fresh = append(fresh, f)
		}
	}
	return fresh
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

// findingKey 生成 finding 去重键(Finding.ID 唯一,最稳)。
func findingKey(f security.Finding) string {
	return f.ID
}
