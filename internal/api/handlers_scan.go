package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/history"
	"code-agent-sentinel/internal/scan"
	"code-agent-sentinel/internal/security"
)

// AgentScanResult 是多 agent 循环扫描中单个 agent 的结果。
// 成功时含 ScanResult 摘要字段;失败时 Error 非空,其余字段为零值。
// 注:跨 agent 不聚合健康分(每 agent 独立评分),前端按需自取 results[i].HealthScore。
type AgentScanResult struct {
	AgentID     string                `json:"agent_id"`
	ScanID      string                `json:"scan_id,omitempty"`
	Findings    []security.Finding    `json:"findings,omitempty"`
	HealthScore *security.HealthScore `json:"health_score,omitempty"`
	Count       int                   `json:"finding_count,omitempty"`
	Error       string                `json:"error,omitempty"`
}

func (s *Server) postScan(c *gin.Context) {
	var ids []string
	if d := c.Query("detectors"); d != "" {
		ids = strings.Split(d, ",")
	}

	// ?agents= 新参数(优先,逗号分隔多 agent);?agent= 旧参数(兼容单 agent)。
	agentsParam := c.Query("agents")
	var agentIDs []string
	if agentsParam != "" {
		agentIDs = strings.Split(agentsParam, ",")
	} else if a := c.Query("agent"); a != "" {
		agentIDs = []string{a}
	}
	// 空 agentIDs → 扫所有「扫描开关开」的 agent。
	// 以 s.Agents(权威注册列表)为准,按 config.AgentCfg.ScanEnabledEffective 过滤
	// (nil → true,向后兼容)。用 s.Agents 而非 Config.ResolveScanAgents 的原因:
	// s.Agents 是 main.go 从 ResolveAgents 构造的真实列表,二者在生产同步;
	// 但测试 fixture 可能只注入 s.Agents 而 Config.Agents 为空(如 newTwoAgentTestServer),
	// 此时 ResolveScanAgents 回退到默认 claude-code 单项,与 s.Agents 不同步 → 误报 unknown_agent。
	// 遍历 s.Agents + 查 Config.ScanEnabled 保持与 getAgents 相同的过滤语义。
	if len(agentIDs) == 0 {
		for _, a := range s.Agents {
			if s.agentScanEnabled(a.ID) {
				agentIDs = append(agentIDs, a.ID)
			}
		}
	}
	// 校验每个 agent 合法性:未知 agent → 400 unknown_agent(不静默回退)。
	for _, id := range agentIDs {
		if !s.agentExists(id) {
			c.JSON(http.StatusBadRequest, errorBody("unknown_agent", "未知 agent: "+id))
			return
		}
	}

	// scope:global(缺省)/project/asset;project/asset 需 path
	scopeType := c.DefaultQuery("scope", "global")
	scopePath := c.Query("path")
	if (scopeType == "project" || scopeType == "asset") && scopePath == "" {
		c.JSON(http.StatusBadRequest, errorBody("bad_request", scopeType+" scope 需 path 参数"))
		return
	}
	if scopeType != "global" && scopeType != "project" && scopeType != "asset" {
		c.JSON(http.StatusBadRequest, errorBody("bad_scope", "未知 scope: "+scopeType))
		return
	}
	scope := scan.ScanScope{Type: scopeType, Path: scopePath}

	// 生成共享 batchID:时间戳 + 4 字节随机 hex(同秒多次扫描不冲突)。
	// 一次 POST /api/scan?agents=a,b,c 的所有 agent 共享同一 batchID,
	// 便于历史按批次聚合(Task 3 的 BatchID 字段)。
	batchID := time.Now().Format("2006-01-02-15-04-05") + "-" + func() string {
		b := make([]byte, 4)
		rand.Read(b)
		return hex.EncodeToString(b)
	}()

	results := make([]AgentScanResult, 0, len(agentIDs))
	for _, aid := range agentIDs {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
		res, err := s.Runner.RunScan(ctx, aid, scope, ids, batchID)
		cancel()
		if err != nil {
			// 单 agent 失败不中断整批:记录 error,继续下一个 agent,整体仍 200。
			results = append(results, AgentScanResult{AgentID: aid, Error: err.Error()})
			continue
		}
		ar := AgentScanResult{AgentID: aid, Findings: res.Findings, Count: len(res.Findings), HealthScore: res.HealthScore}
		if res.StartedAt != (time.Time{}) {
			ar.ScanID = res.StartedAt.Format("2006-01-02-15-04-05")
		}
		results = append(results, ar)
	}
	c.JSON(http.StatusOK, results)
}

func (s *Server) getScanResult(c *gin.Context) {
	agentID := c.Query("agent")
	latest := s.latestScan(agentID)
	if latest == nil {
		c.JSON(http.StatusOK, struct{}{})
		return
	}
	c.JSON(http.StatusOK, latest)
}

// latestScan 返回指定 agent 最近一次扫描的完整记录;空 agentID 退化为全局最新。
// 无历史或 History 未配置返回 nil。
func (s *Server) latestScan(agentID string) *history.ScanRecord {
	if s.History == nil {
		return nil
	}
	latest, err := s.History.LatestForAgent(agentID)
	if err != nil || latest == nil {
		return nil
	}
	return latest
}

// agentScanEnabled 返回 agentID 的扫描开关状态(nil → true,向后兼容旧配置)。
// 与 getAgents 的 ScanEnabled 查找逻辑一致:从 s.Config.Agents 按 ID 查 AgentCfg,
// ScanEnabled nil 默认 true。
func (s *Server) agentScanEnabled(agentID string) bool {
	for _, ac := range s.Config.Agents {
		if ac.ID == agentID {
			return ac.ScanEnabledEffective()
		}
	}
	// Config.Agents 无此 agent(旧配置/测试 fixture)→ 默认开扫
	return true
}
