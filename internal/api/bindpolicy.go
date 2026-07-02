package api

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"code-agent-sentinel/internal/config"
	"github.com/gin-gonic/gin"
)

func isLoopback(addr string) bool {
	return addr == "127.0.0.1" || addr == "localhost" || addr == "::1"
}

// ValidateBindPolicy 校验 bind 策略。
func ValidateBindPolicy(cfg *config.Config, overrideRisky bool) error {
	if isLoopback(cfg.Bind) {
		return nil
	}
	if len(cfg.AllowedCIDRs) == 0 && !overrideRisky {
		return fmt.Errorf("bind=%s 非 loopback 但 allowed_cidrs 为空;出于安全拒绝启动。如确需暴露,请设置 allowed_cidrs 或加 --i-know-its-risky", cfg.Bind)
	}
	return nil
}

// ResolveListenAddr 返回 "bind:port"(port=0 让系统分配)。
func ResolveListenAddr(cfg *config.Config) string {
	return fmt.Sprintf("%s:%d", cfg.Bind, cfg.Port)
}

// clientIPGuard 校验真实客户端 IP 是否在白名单。
func clientIPGuard(cidrs []string) gin.HandlerFunc {
	nets := parseCIDRs(cidrs)
	return func(c *gin.Context) {
		if len(nets) == 0 {
			c.Next()
			return
		}
		ip := net.ParseIP(strings.Split(c.ClientIP(), ":")[0])
		ok := false
		for _, n := range nets {
			if n.Contains(ip) {
				ok = true
				break
			}
		}
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, errorBody("forbidden", "client IP not in allowlist"))
			return
		}
		c.Next()
	}
}

func parseCIDRs(cidrs []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, c := range cidrs {
		if !strings.Contains(c, "/") {
			c += "/32"
		}
		if _, n, err := net.ParseCIDR(c); err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}
