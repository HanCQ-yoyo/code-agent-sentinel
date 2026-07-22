package api

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/configengine"
)

// agentExists 判断 id 是否在 s.Agents 中。空串不是合法 agent ID(返回 false)。
func (s *Server) agentExists(id string) bool {
	if id == "" {
		return false
	}
	for _, a := range s.Agents {
		if a.ID == id {
			return true
		}
	}
	return false
}

// engineForQuery 取请求要用的 agent 的 Engine 与 agentID。
// 优先 ?agent= query;否则 Server.SelectedAgentID;再否则首 agent(空 → s.Agents[0].ID)。
// agentID 非空且不在 s.Agents 时返回错误——调用方报 400(消除未知 agent 静默回退)。
// 空串(无 query 且无 SelectedAgentID)合法:回退首 agent,向后兼容无 ?agent= 调用。
//
// 注:agentID 非空但未知时返回错误,而非传空给 EngineFor——后者会静默回退首 agent,
// 使拼写错的 agent ID 被误当首 agent 处理(读出错误 agent 的数据)。
//
// 返回的 agentID 始终是最终使用的 agent ID(空 → 首 agent ID),便于调用方记日志/写历史。
func (s *Server) engineForQuery(c *gin.Context) (*configengine.Engine, string, error) {
	id := c.Query("agent")
	if id == "" {
		id = s.SelectedAgentID
	}
	if id != "" && !s.agentExists(id) {
		return nil, "", fmt.Errorf("unknown agent: %s", id)
	}
	// id 为空时回退首 agent(NewRunner.EngineFor 内部也做同样兜底,
	// 但这里显式 resolve 使返回的 agentID 非空,便于调用方记日志/写历史)。
	if id == "" && len(s.Agents) > 0 {
		id = s.Agents[0].ID
	}
	return s.Runner.EngineFor(id), id, nil
}

// agentName 返回 agentID 的展示名;未知返回 id 本身。
func (s *Server) agentName(agentID string) string {
	for _, a := range s.Agents {
		if a.ID == agentID {
			return a.Name
		}
	}
	return agentID
}

// agentIDForRequest 取请求要用的 agent ID(不校验合法性,不返回 Engine)。
// 优先 ?agent= query;否则 Server.SelectedAgentID;再否则空串。
// 与 engineForQuery 的差异:engineForQuery 校验未知 agent(报 400),返回 Engine + agentID;
// agentIDForRequest 不校验——未知 agent 经 Runner.EngineFor 兜底回退首 agent,
// 用于 partialRescan 容错路径(编辑已落盘成功,不应因 agent 拼写错让 rescan 报错)。
//
// 用途:commitAsset → partialRescan 需要一个 agentID 给 Runner.RunScan,但编辑路径
// 不强制 400 未知 agent(写入已成功,rescan 失败可降级提示)。未知 agent → EngineFor
// 回退首 agent → 扫首 agent 的资产(可能与被编辑资产不同 agent,但 rescan 本就 best-effort)。
func (s *Server) agentIDForRequest(c *gin.Context) string {
	id := c.Query("agent")
	if id == "" {
		id = s.SelectedAgentID
	}
	return id
}
