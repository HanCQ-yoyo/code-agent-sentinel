// internal/api/handlers_allowlist.go
package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// allowlistBody 是 GET/PUT /api/guard/allowlist 的 JSON 结构。
type allowlistBody struct {
	Allowlist []string `json:"allowlist"`
}

// getAllowlist 返回当前放行清单。nil-safe:Allowlist==nil 返回空列表(非 error),
// 与 putAllowlist 的 nil→503 不对称——GET 是只读探查,空列表等价于"无放行",
// 不应让前端在 sentinel 未配置 allowlist 时报错。
func (s *Server) getAllowlist(c *gin.Context) {
	if s.Allowlist == nil {
		c.JSON(http.StatusOK, allowlistBody{Allowlist: []string{}})
		return
	}
	list, err := s.Allowlist.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("allowlist_load_failed", err.Error()))
		return
	}
	if list == nil {
		list = []string{}
	}
	c.JSON(http.StatusOK, allowlistBody{Allowlist: list})
}

// putAllowlist 全量替换放行清单 + 原子写(Task 4 Save:tmp+rename)。
//
// 顶层键校验(防部分体):镜像 putGuardConfig 的两遍解码——先 map 确认 "allowlist"
// 顶层键存在,再结构体解码。allowlist 字段是 []string(切片),JSON 反序列化缺失键
// 与显式 null/[] 都是 nil/空切片,语义上无法区分"清空"与"漏发";强制要求顶层键
// 存在使得客户端必须显式发送 {"allowlist":[]} 才能清空,避免漏发键导致误清空。
//
// nil-safe:Allowlist==nil 返回 503(放行清单未配置,写无意义;与 GET 不对称,
// 因为写需要持久化路径,无路径无法落盘)。
func (s *Server) putAllowlist(c *gin.Context) {
	if s.Allowlist == nil {
		c.JSON(http.StatusServiceUnavailable, errorBody("allowlist_disabled", "allowlist store not configured"))
		return
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorBody("invalid_body", err.Error()))
		return
	}
	// 两遍解码:先 map 检查顶层键齐全(防部分体静默清空),再结构体。
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("invalid_body", err.Error()))
		return
	}
	if _, ok := top["allowlist"]; !ok {
		c.JSON(http.StatusBadRequest, errorBody("invalid_body",
			"missing allowlist key; send {\"allowlist\":[...]} (空数组=清空)"))
		return
	}
	var body allowlistBody
	if err := json.Unmarshal(raw, &body); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("invalid_body", err.Error()))
		return
	}
	if body.Allowlist == nil {
		body.Allowlist = []string{}
	}
	if err := s.Allowlist.Save(body.Allowlist); err != nil {
		c.JSON(http.StatusInternalServerError, errorBody("save_failed", err.Error()))
		return
	}
	c.JSON(http.StatusOK, allowlistBody{Allowlist: body.Allowlist})
}
