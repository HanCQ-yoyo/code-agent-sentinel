package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuthMiddlewareRejectsMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(authMiddleware("secret"))
	r.GET("/api/x", func(c *gin.Context) { c.String(200, "ok") })
	req := httptest.NewRequest("GET", "/api/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("无 token 应 401, got %d", w.Code)
	}
}

func TestAuthMiddlewareAcceptsBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(authMiddleware("secret"))
	r.GET("/api/x", func(c *gin.Context) { c.String(200, "ok") })
	req := httptest.NewRequest("GET", "/api/x", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("应 200, got %d", w.Code)
	}
}

func TestHostMiddlewareRejectsBadHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(hostMiddleware([]string{"127.0.0.1"}))
	r.GET("/api/x", func(c *gin.Context) { c.String(200, "ok") })
	req := httptest.NewRequest("GET", "/api/x", nil)
	req.Host = "evil.com"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("坏 Host 应 403, got %d", w.Code)
	}
}
