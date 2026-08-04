// internal/api/handlers_allowlist_test.go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/storage"
)

// newAllowlistTestServer 构造仅 Allowlist + Config 就绪的 Server(handler 单测足够;
// 不走 NewServer,避免拉起 Engine/Orchestrator/Runner 等重型依赖)。
func newAllowlistTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "allowlist.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s := &Server{}
	s.Config = newTestConfig()
	s.Allowlist = config.NewAllowlistStore(db)
	return s
}

func TestGetAllowlistEmpty(t *testing.T) {
	s := newAllowlistTestServer(t)
	c, w := newTestContext(httptest.NewRequest("GET", "/api/guard/allowlist", nil))
	s.getAllowlist(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var got struct {
		Allowlist []string `json:"allowlist"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, w.Body.String())
	}
	if len(got.Allowlist) != 0 {
		t.Fatalf("空应返回空列表: %v", got.Allowlist)
	}
}

func TestPutAllowlistFullReplace(t *testing.T) {
	s := newAllowlistTestServer(t)
	body := `{"allowlist":["rm -rf node_modules","git clean -fdx dist"]}`
	c, w := newTestContext(httptest.NewRequest("PUT", "/api/guard/allowlist", strings.NewReader(body)))
	s.putAllowlist(c)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT status=%d body=%s", w.Code, w.Body.String())
	}
	// 验证落盘(DB 按 ORDER BY command 排序,故列表为字母序)
	list, err := s.Allowlist.Load()
	if err != nil {
		t.Fatalf("Load err: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("未落盘: %v", list)
	}
	// 元素验证(顺序为字母序)
	found := make(map[string]bool, len(list))
	for _, l := range list {
		found[l] = true
	}
	for _, want := range []string{"rm -rf node_modules", "git clean -fdx dist"} {
		if !found[want] {
			t.Fatalf("缺失 %q: %v", want, list)
		}
	}
}

func TestPutAllowlistMissingKey(t *testing.T) {
	s := newAllowlistTestServer(t)
	body := `["rm -rf node_modules"]` // 缺 allowlist 顶层键
	c, w := newTestContext(httptest.NewRequest("PUT", "/api/guard/allowlist", strings.NewReader(body)))
	s.putAllowlist(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺顶层键应 400: %d body=%s", w.Code, w.Body.String())
	}
}

// TestGetAllowlistNilStore 验证 nil-safe:Allowlist==nil 返回空列表(非 503)。
func TestGetAllowlistNilStore(t *testing.T) {
	s := &Server{Config: newTestConfig()} // Allowlist 未设置
	c, w := newTestContext(httptest.NewRequest("GET", "/api/guard/allowlist", nil))
	s.getAllowlist(c)
	if w.Code != http.StatusOK {
		t.Fatalf("nil store GET 应 200(返回空列表): %d", w.Code)
	}
	var got struct {
		Allowlist []string `json:"allowlist"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Allowlist) != 0 {
		t.Fatalf("nil store 应返回空列表: %v", got.Allowlist)
	}
}

// TestPutAllowlistNilStore 验证 nil store PUT 返回 503。
func TestPutAllowlistNilStore(t *testing.T) {
	s := &Server{Config: newTestConfig()} // Allowlist 未设置
	body := `{"allowlist":["ls"]}`
	c, w := newTestContext(httptest.NewRequest("PUT", "/api/guard/allowlist", strings.NewReader(body)))
	s.putAllowlist(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil store PUT 应 503: %d", w.Code)
	}
}

// TestGuardConfigSnapshotIncludesR3 验证 guard/config 快照含 mode/allowlist_enabled。
func TestGuardConfigSnapshotIncludesR3(t *testing.T) {
	s := newAllowlistTestServer(t)
	c, w := newTestContext(httptest.NewRequest("GET", "/api/guard/config", nil))
	s.getGuardConfig(c)
	if w.Code != http.StatusOK {
		t.Fatalf("GET status=%d", w.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := got["mode"]; !ok {
		t.Fatalf("snapshot 缺 mode: %v", got)
	}
	if _, ok := got["allowlist_enabled"]; !ok {
		t.Fatalf("snapshot 缺 allowlist_enabled: %v", got)
	}
}

// TestPutGuardConfigRejectsPartialR3 验证 PUT /guard/config 缺 mode/allowlist_enabled 被拒。
func TestPutGuardConfigRejectsPartialR3(t *testing.T) {
	s := newAllowlistTestServer(t)
	s.ConfigPath = filepath.Join(t.TempDir(), "config.yaml")
	// 缺 mode 与 allowlist_enabled
	body := `{"enabled":true,"policy":"deny","deadline_ms":200,"max_command_bytes":1024}`
	c, w := newTestContext(httptest.NewRequest("PUT", "/api/guard/config", strings.NewReader(body)))
	s.putGuardConfig(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺 mode/allowlist_enabled 应 400: %d body=%s", w.Code, w.Body.String())
	}
}

// TestPutGuardConfigAcceptsFullR3 验证 PUT /guard/config 含全部 6 键成功。
func TestPutGuardConfigAcceptsFullR3(t *testing.T) {
	s := newAllowlistTestServer(t)
	s.ConfigPath = filepath.Join(t.TempDir(), "config.yaml")
	body := `{"enabled":true,"policy":"deny","deadline_ms":200,"max_command_bytes":1024,"mode":"lenient","allowlist_enabled":false}`
	c, w := newTestContext(httptest.NewRequest("PUT", "/api/guard/config", strings.NewReader(body)))
	s.putGuardConfig(c)
	if w.Code != http.StatusOK {
		t.Fatalf("全键 PUT 应 200: %d body=%s", w.Code, w.Body.String())
	}
	if s.Config.Guard.Mode != "lenient" || s.Config.Guard.AllowlistEnabled != false {
		t.Fatalf("R3 字段未生效: mode=%q allowlist_enabled=%v",
			s.Config.Guard.Mode, s.Config.Guard.AllowlistEnabled)
	}
}
