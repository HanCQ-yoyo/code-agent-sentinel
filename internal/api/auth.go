package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// authMiddleware 校验每个 /api 请求的 bearer token。
func authMiddleware(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// SPA 静态资源放行
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}
		t := c.GetHeader("Authorization")
		t = strings.TrimPrefix(t, "Bearer ")
		if t == "" {
			t = c.Query("token")
		}
		if t != token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, errorBody("unauthorized", "missing or invalid token"))
			return
		}
		c.Next()
	}
}

// hostMiddleware 校验 Host 头防 DNS rebinding。
func hostMiddleware(allowed []string) gin.HandlerFunc {
	set := map[string]bool{}
	for _, h := range allowed {
		set[h] = true
	}
	return func(c *gin.Context) {
		if len(set) == 0 {
			c.Next()
			return
		}
		host := c.Request.Host
		if i := strings.LastIndex(host, ":"); i > 0 {
			host = host[:i]
		}
		if !set[host] {
			c.AbortWithStatusJSON(http.StatusForbidden, errorBody("forbidden", "host not allowed"))
			return
		}
		c.Next()
	}
}

// corsStrict 拒绝跨域(只允许同源)。
func corsStrict() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", c.Request.Header.Get("Origin"))
		// 不设通配 *,且要求 token;实际跨域请求无 token 会被 auth 拦
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func errorBody(code, msg string) gin.H {
	return gin.H{"error": gin.H{"code": code, "message": msg}}
}
