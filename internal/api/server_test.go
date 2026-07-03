package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security"
)

// TestRouterXFFNotTrusted 验证 C-SEC-1:SetTrustedProxies(nil) 后,
// 攻击者伪造的 X-Forwarded-For 不影响 c.ClientIP() 解析的真实客户端 IP,
// 从而无法绕过 IP 白名单。
//
// 场景:白名单 10.0.0.0/8;真实 RemoteAddr 192.168.1.5(不在白名单);
// 伪造 XFF=10.5.5.5(在白名单内)。
//   - 旧实现(gin 默认信任 0.0.0.0/0):ClientIP=10.5.5.5 → 白名单放行(200,绕过!)
//   - 修复后(SetTrustedProxies(nil)):ClientIP=192.168.1.5 → 白名单拒绝(403)
//
// 探测端点挂在 /probe(非 /api/)以绕过 authMiddleware,但 hostMiddleware 与
// clientIPGuard 对非 loopback 绑定应用于全部路由,故仍能验证 guard 行为。
func TestRouterXFFNotTrusted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{Bind: "0.0.0.0", AllowedCIDRs: []string{"10.0.0.0/8"}}
	srv := NewServer(&configengine.Engine{}, &security.Orchestrator{}, cfg, "tok")
	r := srv.Router()
	r.GET("/probe", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("GET", "/probe", nil)
	req.Host = "0.0.0.0" // 命中 hostMiddleware 允许列表(Bind 非 loopback 时加入)
	req.RemoteAddr = "192.168.1.5:54321" // 真实 IP,不在白名单
	req.Header.Set("X-Forwarded-For", "10.5.5.5") // 伪造,在白名单内
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("伪造 XFF 不应绕过 IP 白名单:期望 403(真实 IP 192.168.1.5 不在白名单),得到 %d (body=%s)", w.Code, w.Body.String())
	}
}
