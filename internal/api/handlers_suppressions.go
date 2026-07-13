package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/security/suppression"
)

// suppressionItemResponse 是 GET /api/suppressions 返回的单条豁免项。
// ID 是 DELETE /api/suppressions/:id 用的稳定标识符,由字段计算得出(见 computeSuppressionID)。
type suppressionItemResponse struct {
	ID          string `json:"id"`
	Fingerprint string `json:"fingerprint"`
	RuleID      string `json:"rule_id"`
	AssetID     string `json:"asset_id"`
	Reason      string `json:"reason"`
}

// suppressionsResponse 是 GET /api/suppressions 的响应体。
type suppressionsResponse struct {
	Items []suppressionItemResponse `json:"items"`
}

// postSuppressionBody 是 POST /api/suppressions 的请求体。
// 三档由字段填充情况决定(同 suppression.Item):
//   - Fingerprint 非空 → 指纹档(最高优先级)
//   - RuleID + AssetID 非空 → rule+asset 档
//   - 仅 RuleID 非空 → rule 全局档
type postSuppressionBody struct {
	Fingerprint string `json:"fingerprint"`
	RuleID      string `json:"rule_id"`
	AssetID     string `json:"asset_id"`
	Reason      string `json:"reason"`
}

// computeSuppressionID 由豁免项字段计算稳定标识符(不依赖列表顺序)。
// 三档分别加前缀防跨档碰撞:
//   - "fp:" + Fingerprint
//   - "ra:" + RuleID + ":" + AssetID
//   - "rg:" + RuleID
//
// DELETE /api/suppressions/:id 用此 id 定位条目:GET 列表时返回 id,前端拿 id 调 DELETE,
// handler 逐条重算 id 比对定位。id 稳定(同一 item 字段不变则 id 不变),跨删除安全。
func computeSuppressionID(item suppression.Item) string {
	if item.Fingerprint != "" {
		return "fp:" + item.Fingerprint
	}
	if item.RuleID != "" && item.AssetID != "" {
		return "ra:" + item.RuleID + ":" + item.AssetID
	}
	if item.RuleID != "" {
		return "rg:" + item.RuleID
	}
	return ""
}

// postSuppression 添加一条豁免规则,写入 suppressions.yaml(0o600)。
// 路径由 s.Config.ResolveSuppressPath(home) 解析(支持 config 覆盖)。
func (s *Server) postSuppression(c *gin.Context) {
	var body postSuppressionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", err.Error()))
		return
	}
	// 校验:至少 Fingerprint 或 RuleID 之一非空(否则无法匹配任何档)。
	if body.Fingerprint == "" && body.RuleID == "" {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", "fingerprint 或 rule_id 至少需填一个"))
		return
	}

	path := s.Config.ResolveSuppressPath(s.Engine.HomeDir)
	supprs, err := suppression.LoadSuppressions(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("load_failed", err.Error()))
		return
	}
	if supprs == nil {
		supprs = &suppression.Suppressions{}
	}

	item := suppression.Item{
		Fingerprint: body.Fingerprint,
		RuleID:      body.RuleID,
		AssetID:     body.AssetID,
		Reason:      body.Reason,
	}
	supprs.Items = append(supprs.Items, item)

	if err := supprs.Save(path); err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, suppressionItemResponse{
		ID:          computeSuppressionID(item),
		Fingerprint: item.Fingerprint,
		RuleID:      item.RuleID,
		AssetID:     item.AssetID,
		Reason:      item.Reason,
	})
}

// getSuppressions 返回全部豁免规则列表。
func (s *Server) getSuppressions(c *gin.Context) {
	path := s.Config.ResolveSuppressPath(s.Engine.HomeDir)
	supprs, err := suppression.LoadSuppressions(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("load_failed", err.Error()))
		return
	}
	items := make([]suppressionItemResponse, 0)
	if supprs != nil {
		for _, item := range supprs.Items {
			items = append(items, suppressionItemResponse{
				ID:          computeSuppressionID(item),
				Fingerprint: item.Fingerprint,
				RuleID:      item.RuleID,
				AssetID:     item.AssetID,
				Reason:      item.Reason,
			})
		}
	}
	c.JSON(http.StatusOK, suppressionsResponse{Items: items})
}

// deleteSuppression 按 id 删除一条豁免规则。
// id 是 computeSuppressionID 计算的稳定标识符(GET 列表返回的 id)。
// handler 逐条重算 id 比对定位,删除后回写文件。
func (s *Server) deleteSuppression(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", "missing id"))
		return
	}

	path := s.Config.ResolveSuppressPath(s.Engine.HomeDir)
	supprs, err := suppression.LoadSuppressions(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("load_failed", err.Error()))
		return
	}
	if supprs == nil {
		c.JSON(http.StatusNotFound, errorBody("not_found", "suppression not found"))
		return
	}

	found := false
	kept := make([]suppression.Item, 0, len(supprs.Items))
	for _, item := range supprs.Items {
		if computeSuppressionID(item) == id {
			found = true
			continue
		}
		kept = append(kept, item)
	}
	if !found {
		c.JSON(http.StatusNotFound, errorBody("not_found", "suppression not found"))
		return
	}

	supprs.Items = kept
	if err := supprs.Save(path); err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// postBaseline 跑一次全量扫描,收集所有 Finding 的 fingerprint,
// 合并到现有 baseline(union:保留已有 + 添加新发现),保存到 baseline.json(0o600)。
// 不做 prune(清理不复现指纹是 CLI --prune 的职责)。
func (s *Server) postBaseline(c *gin.Context) {
	inv, err := s.Engine.Discover()
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
	newFPs := make(map[string]bool)
	for _, f := range res.Findings {
		if f.Fingerprint != "" {
			newFPs[f.Fingerprint] = true
		}
	}

	path := s.Config.ResolveBaselinePath(s.Engine.HomeDir)
	existing, err := suppression.LoadBaseline(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("load_failed", err.Error()))
		return
	}

	// 合并:union(保留已有 + 添加新发现)。
	var bs *suppression.BaselineSet
	added := 0
	if existing != nil {
		bs = existing
		if bs.Fingerprints == nil {
			bs.Fingerprints = make(map[string]bool)
		}
		for fp := range newFPs {
			if !bs.Fingerprints[fp] {
				bs.Fingerprints[fp] = true
				added++
			}
		}
	} else {
		bs = &suppression.BaselineSet{
			Version:      "1",
			Fingerprints: newFPs,
		}
		added = len(newFPs)
	}
	bs.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	if err := bs.Save(path); err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"baseline_path": path,
		"total_fps":     len(bs.Fingerprints),
		"added_fps":     added,
		"scan_findings": len(res.Findings),
	})
}
