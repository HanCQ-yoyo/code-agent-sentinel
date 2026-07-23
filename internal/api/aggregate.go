package api

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// resolveAgentIDs 解析 ?agent= query 返回 agent ID 列表。
// "all" / 空 → 所有 s.Agents(按注册顺序);单 ID → []string{id};
// 多 ID(逗号分隔) → []string{id1,id2,...}(去空白、去空串)。
//
// 与 engineForQuery 的关键差异:resolveAgentIDs 不回退 SelectedAgentID 或首 agent。
// ?agent=all 与空 query 都意味着"所有 agent";单个显式 ID 意味着 [thatID]。
// 调用方据此自行校验 agent 合法性并决定聚合/单 agent 路径。
func (s *Server) resolveAgentIDs(c *gin.Context) []string {
	q := c.Query("agent")
	if q == "" || q == "all" {
		out := make([]string, len(s.Agents))
		for i, a := range s.Agents {
			out[i] = a.ID
		}
		return out
	}
	parts := strings.Split(q, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// shouldAggregate 判断当前请求是否应走聚合路径。
// 规则:?agent=all 显式请求聚合(即使最终只解析出 1 个 agent,也走聚合);
// 否则当 resolveAgentIDs 返回的列表长度 != 1 时聚合(0 或 ≥2)。
//
// 此判断独立于 resolveAgentIDs 的返回值长度,因为单 agent fixture 上 ?agent=all
// 会解析出 ["claude-code"](len==1),但用户显式请求了"全部"→ 仍应走聚合路径
// (返回 agent_scans 数组,而非单 agent 的 agent/agent_name 字段)。
// 这是与 brief 中纯计数式 isAggregation 的有意偏差,使 ?agent=all 语义稳定。
func shouldAggregate(c *gin.Context, agentIDs []string) bool {
	if c.Query("agent") == "all" {
		return true
	}
	return len(agentIDs) != 1
}
