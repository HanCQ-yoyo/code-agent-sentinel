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
	"code-agent-sentinel/internal/editor"
	"code-agent-sentinel/internal/history"
	"code-agent-sentinel/internal/security"
	"code-agent-sentinel/internal/security/ruleengine"
	"code-agent-sentinel/internal/storage"
)

// newTestDB 是 Task 12 提取的共享 db 构造助手:
//  1. 在 home 下开临时 db 文件,RunMigrations 建表;
//  2. LoadBuiltin + SyncBuiltin 把 embed 内置规则同步进 detect + intercept 两域(版本 v1);
//  3. 注册 t.Cleanup(db.Close)。
//
// newTestServer / newTestServerWithAgents / newServerWithRulesDB 共用此 helper,
// 确保规则 API 测试 + 普通测试 + handler 测试看到的规则库一致。镜像
// cmd/sentinel/syncBuiltinRules 的语义(detect 域带 combos,intercept 域只带规则)。
func newTestDB(t *testing.T, home string) *storage.DB {
	t.Helper()
	dbPath := filepath.Join(home, "test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := storage.RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	// 同步 builtin 规则进两域(镜像 cmd/sentinel/syncBuiltinRules)。
	builtin, builtinCombos, loadErrs := ruleengine.LoadBuiltin()
	for _, e := range loadErrs {
		t.Logf("builtin load err %s: %s", e.Source, e.Reason)
	}
	builtinStored := make([]storage.StoredRule, 0, len(builtin))
	for _, r := range builtin {
		s, convErr := ruleengine.RuleToStoredRule(r, "builtin", "v1")
		if convErr != nil {
			t.Fatalf("convert builtin rule %s: %v", r.ID, convErr)
		}
		builtinStored = append(builtinStored, s)
	}
	builtinComboStored := make([]storage.StoredCombo, 0, len(builtinCombos))
	for _, c := range builtinCombos {
		s, convErr := ruleengine.ComboToStoredCombo(c, "builtin", "v1")
		if convErr != nil {
			t.Fatalf("convert builtin combo %s: %v", c.ID, convErr)
		}
		builtinComboStored = append(builtinComboStored, s)
	}
	if _, err := storage.SyncBuiltin(db, storage.DomainDetect, builtinStored, builtinComboStored, "v1"); err != nil {
		t.Fatalf("sync builtin detect: %v", err)
	}
	if _, err := storage.SyncBuiltin(db, storage.DomainIntercept, builtinStored, nil, "v1"); err != nil {
		t.Fatalf("sync builtin intercept: %v", err)
	}
	return db
}

func newTestServer(t *testing.T, home string) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	eng := configengine.NewEngine(home, "")
	// Task 12:注入真实 test db(而非 nil),使 /api/detect-rules 等
	// 规则路由经 newTestServer 即可访问 builtin 规则,无需 newServerWithRulesDB。
	db := newTestDB(t, home)
	r := security.NewRegistry()
	r.Register(security.NewRulesDetector(home, nil, db))
	orch := &security.Orchestrator{Registry: r}
	hist := history.NewStore(db)
	ed := editor.New(eng, "", 0)
	return NewServer(eng, orch, config.DefaultConfig(), "tok", hist, configengine.DefaultAgents(home, ""), ed, db)
}

// newTestServerWithAgents 用指定 agents 构造 Server(多 agent fixture 用)。
// 与 newTestServer 的差异:agents 显式传入(而非 DefaultAgents 单 agent),
// NewServer 内部用此 agents 列表构造 scan.Runner,EngineFor 按 agentID 池化。
// hist 目录同样放在 home 外(与 .claude 同级)避免 configengine 扫到。
// SelectedAgentID 默认置为 agents[0].ID(NewServer 既有行为)。
// Task 12:同样注入真实 test db(规则路由可用,保持与 newTestServer 对称)。
func newTestServerWithAgents(t *testing.T, eng *configengine.Engine, agents []configengine.Agent) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := newTestDB(t, eng.HomeDir)
	r := security.NewRegistry()
	r.Register(security.NewRulesDetector(eng.HomeDir, nil, db))
	orch := &security.Orchestrator{Registry: r}
	hist := history.NewStore(db)
	ed := editor.New(eng, "", 0)
	return NewServer(eng, orch, config.DefaultConfig(), "tok", hist, agents, ed, db)
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

	// 从 embed 中找一个真实的 .js 资源名(文件名带 hash,不能硬编码)。
	// 新检出/CI 未跑 make web 时,web_dist 只有占位 index.html、无 assets/ 目录,
	// 此处跳过 .js 资源断言;其余三处(/、/dashboard、/api/nonexistent)对占位
	// index.html 仍成立,必须继续执行。
	entries, err := webFS.ReadDir("web_dist/assets")
	jsName := ""
	if err != nil {
		t.Logf("read web_dist/assets: %v(web_dist 无构建产物,运行 make web;跳过 .js 资源断言)", err)
	} else {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".js") {
				jsName = e.Name()
				break
			}
		}
		if jsName == "" {
			t.Logf("no .js asset in web_dist/assets(web_dist 无构建产物,运行 make web;跳过 .js 资源断言)")
		}
	}

	if jsName != "" {
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
