// internal/api/handlers_intercept_test.go
package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/intercept"
	"code-agent-sentinel/internal/storage"
)

// newTestConfig 返回带 Guard 非-nil 默认配置(guard/intercept handler 测试用)。
func newTestConfig() *config.Config {
	c := config.DefaultConfig()
	c.EnsureGuard()
	return c
}

// newTestInterceptServer 构造仅 Intercept + Config 就绪的 Server(handler 单测足够;
// 不走 NewServer,避免拉起 Engine/Orchestrator/Runner 等重型依赖)。
// ConfigPath 指向临时目录,使 PUT /api/guard/config 的 config.Save 可写盘成功。
func newTestInterceptServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := storage.RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	s := &Server{Intercept: intercept.NewStore(db)}
	s.Config = newTestConfig()
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	return s
}

// newTestContext 构造一个绑定到 httptest.ResponseRecorder 的 *gin.Context,
// 并把 req 挂上去(含 body / path value)。供 handler 单测直接调用 s.<handler>(c) 用。
func newTestContext(req *http.Request) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, w
}

func TestGetInterceptList(t *testing.T) {
	s := newTestInterceptServer(t)
	s.Intercept.Append(intercept.InterceptRecord{ID: "r1", Outcome: "deny", Command: "rm -rf /"})
	s.Intercept.Append(intercept.InterceptRecord{ID: "r2", Outcome: "allow", Command: "ls"})
	c, w := newTestContext(httptest.NewRequest("GET", "/api/intercept", nil))
	s.getInterceptList(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if !contains(w.Body.String(), "rm -rf /") {
		t.Fatalf("列表应含 rm -rf /: %s", w.Body.String())
	}
}

func TestGetInterceptDetail(t *testing.T) {
	s := newTestInterceptServer(t)
	s.Intercept.Append(intercept.InterceptRecord{ID: "r1", Command: "rm -rf /", Outcome: "deny"})
	req := httptest.NewRequest("GET", "/api/intercept/r1", nil)
	c, w := newTestContext(req)
	c.Params = gin.Params{{Key: "id", Value: "r1"}}
	s.getInterceptDetail(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestDeleteIntercept(t *testing.T) {
	s := newTestInterceptServer(t)
	s.Intercept.Append(intercept.InterceptRecord{ID: "r1", Command: "rm -rf /"})
	req := httptest.NewRequest("DELETE", "/api/intercept/r1", nil)
	c, w := newTestContext(req)
	c.Params = gin.Params{{Key: "id", Value: "r1"}}
	s.deleteIntercept(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestGuardConfigGetPut(t *testing.T) {
	s := newTestInterceptServer(t)
	// GET
	cGet, wGet := newTestContext(httptest.NewRequest("GET", "/api/guard/config", nil))
	s.getGuardConfig(cGet)
	if wGet.Code != http.StatusOK {
		t.Fatalf("GET status=%d", wGet.Code)
	}
	// PUT(Stage R3:须含全部 6 键 enabled/policy/deadline_ms/max_command_bytes/mode/allowlist_enabled)
	body := `{"enabled":false,"policy":"warn","deadline_ms":500,"max_command_bytes":1024,"mode":"strict","allowlist_enabled":true}`
	cPut, wPut := newTestContext(httptest.NewRequest("PUT", "/api/guard/config", strings.NewReader(body)))
	s.putGuardConfig(cPut)
	if wPut.Code != http.StatusOK {
		t.Fatalf("PUT status=%d body=%s", wPut.Code, wPut.Body.String())
	}
	if s.Config.Guard.Enabled != false || s.Config.Guard.DeadlineMS != 500 {
		t.Fatalf("PUT 未生效: %+v", s.Config.Guard)
	}
}
