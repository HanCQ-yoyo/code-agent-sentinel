package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security"
)

func newTestServer(t *testing.T, home string) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	eng := configengine.NewEngine(home)
	r := security.NewRegistry()
	r.Register(security.NewBaselineDetector())
	orch := &security.Orchestrator{Registry: r}
	return NewServer(eng, orch, config.DefaultConfig(), "tok")
}

func TestGetAssets(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), `{"model":"opus"}`)
	s := newTestServer(t, dir)
	r := s.Router()
	req := httptest.NewRequest("GET", "/api/assets", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	var inv configengine.Inventory
	json.Unmarshal(w.Body.Bytes(), &inv)
	if len(inv.Assets) == 0 {
		t.Error("无资产")
	}
}

func TestGetHealthEmpty(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	r := s.Router()
	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestGetDetectors(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	r := s.Router()
	req := httptest.NewRequest("GET", "/api/detectors", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestPostProject(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	r := s.Router()
	req := httptest.NewRequest("POST", "/api/project?path=/tmp/foo", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func writeFile(t *testing.T, p, c string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(c), 0o644)
}

// TestSPAServing 验证 NoRoute 处理器对静态资源与 SPA 路由的行为:
// 真实静态文件按扩展名返回正确 Content-Type;未匹配路径回退 index.html。
// 防止 embed 管线回归(曾出现 JS 资源被当作 HTML 返回的 bug)。
func TestSPAServing(t *testing.T) {
	s := newTestServer(t, t.TempDir())
	r := s.Router()

	// / 应返回 index.html(HTML)
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "127.0.0.1"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("GET /: got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("GET /: Content-Type=%s, want text/html", ct)
	}

	// 从 embed 中找一个真实的 .js 资源名(文件名带 hash,不能硬编码)
	entries, err := webFS.ReadDir("web_dist/assets")
	if err != nil {
		t.Fatalf("read web_dist/assets: %v", err)
	}
	jsName := ""
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".js") {
			jsName = e.Name()
			break
		}
	}
	if jsName == "" {
		t.Fatal("no .js asset in web_dist/assets(运行 make web 后再测试)")
	}

	// /assets/<js> 应返回 JS(不是 HTML 回退)
	req = httptest.NewRequest("GET", "/assets/"+jsName, nil)
	req.Host = "127.0.0.1"
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("GET /assets/%s: got %d", jsName, w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Fatalf("GET /assets/%s: Content-Type=%s, want text/javascript", jsName, ct)
	}
	body := w.Body.String()
	if strings.HasPrefix(body, "<!doctype") {
		t.Fatalf("GET /assets/%s: got HTML fallback, want real JS asset", jsName)
	}

	// /dashboard(SPA 客户端路由)应回退 index.html
	req = httptest.NewRequest("GET", "/dashboard", nil)
	req.Host = "127.0.0.1"
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("GET /dashboard: got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("GET /dashboard: Content-Type=%s, want text/html", ct)
	}

	// /api/ 未知子路径应返回 JSON 404,不是 index.html(需带 token 通过 auth)
	req = httptest.NewRequest("GET", "/api/nonexistent", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("GET /api/nonexistent: got %d, want 404", w.Code)
	}
}
