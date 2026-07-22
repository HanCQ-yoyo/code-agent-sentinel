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
