package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
	"code-agent-sentinel/internal/security"
)

// newConfigTestServer 构造带完整检测器配置的测试服务器。
// 与 newTestServer 的区别:后者只注册 RulesDetector 且不调 EnsureDetectors(cfg.Detectors=nil),
// 无法测试 PUT 原地改写。此 helper 镜像 main.go 接线:EnsureDetectors + 3 个检测器持同一 cfg.Detectors 指针。
func newConfigTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := config.DefaultConfig()
	cfg.EnsureDetectors()
	eng := configengine.NewEngine(dir, "")
	r := security.NewRegistry()
	r.Register(security.NewRulesDetector(dir, cfg.Detectors))
	r.Register(security.NewSecretDetector(cfg.Detectors))
	r.Register(security.NewDependencyDetector(cfg.Detectors))
	orch := &security.Orchestrator{Registry: r}
	srv := NewServer(eng, orch, cfg, "tok", nil, nil, nil)
	srv.ConfigPath = cfgPath
	return srv, cfgPath
}

func TestGetDetectorConfig(t *testing.T) {
	srv, _ := newConfigTestServer(t)
	r := srv.Router()
	req := httptest.NewRequest(http.MethodGet, "/api/detectors/config", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var got config.DetectorsConfig
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	// 默认全启用
	if !got.Rules.Enabled {
		t.Error("默认 Rules.Enabled 应 true")
	}
	if !got.Secret.Enabled {
		t.Error("默认 Secret.Enabled 应 true")
	}
}

func TestPutDetectorConfig(t *testing.T) {
	srv, cfgPath := newConfigTestServer(t)
	r := srv.Router()
	body := `{"rules":{"enabled":false},"secret":{"enabled":true,"binary":"/opt/gitleaks"},"dep":{"enabled":true,"engines":{"npm":{"enabled":false},"govulncheck":{"enabled":true}}}}`
	req := httptest.NewRequest(http.MethodPut, "/api/detectors/config", strings.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	// 运行期配置已原地改写:rules 检测器读到新值。
	// Enabled() 经 Detector 接口导出,可跨包调用(不依赖 unexported binary())。
	rd := srv.Orchestrator.Registry.Get("rules")
	if rd == nil {
		t.Fatal("rules 检测器未注册")
	}
	if rd.Enabled() {
		t.Error("PUT 后 rules 应禁用")
	}
	// 再 GET 一次,确认返回 JSON 反映新值(secret.binary = /opt/gitleaks)
	req2 := httptest.NewRequest(http.MethodGet, "/api/detectors/config", nil)
	req2.Host = "127.0.0.1"
	req2.Header.Set("Authorization", "Bearer tok")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("GET after PUT status = %d, body = %s", w2.Code, w2.Body.String())
	}
	var got config.DetectorsConfig
	if err := json.Unmarshal(w2.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Secret.Binary != "/opt/gitleaks" {
		t.Errorf("PUT 后 secret binary = %q, want /opt/gitleaks", got.Secret.Binary)
	}
	if got.Dep.Engines["npm"].Enabled {
		t.Error("PUT 后 dep npm 引擎应禁用")
	}
	// 持久化:重新加载读到新值(验证 detectors: 段确实写入文件)
	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Detectors == nil {
		t.Fatal("持久化后 Detectors 为 nil(detectors 段未写入)")
	}
	if loaded.Detectors.RulesEnabled() {
		t.Error("持久化后 Rules 应仍禁用")
	}
	if got := loaded.Detectors.SecretBinaryOrDefault(); got != "/opt/gitleaks" {
		t.Errorf("持久化后 secret binary = %q, want /opt/gitleaks", got)
	}
}

// TestPutDetectorConfigRejectsPartialBody 验证部分体(缺 secret/dep 顶层键)被拒绝。
// 安全相关:部分体会因 bool 零值=false 静默禁用未指定的检测器。
func TestPutDetectorConfigRejectsPartialBody(t *testing.T) {
	srv, _ := newConfigTestServer(t)
	r := srv.Router()
	body := `{"rules":{"enabled":false}}`
	req := httptest.NewRequest(http.MethodPut, "/api/detectors/config", strings.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
	var resp map[string]map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"]["code"] != "invalid_config" {
		t.Errorf("error code = %q, want invalid_config; body = %s", resp["error"]["code"], w.Body.String())
	}
}

// TestPutDetectorConfigRejectsMissingDepEngine 验证 dep.engines 须含 npm + govulncheck。
// 缺引擎会因 map 缺键导致该引擎默认启用,但配置不完整——应拒绝。
func TestPutDetectorConfigRejectsMissingDepEngine(t *testing.T) {
	srv, _ := newConfigTestServer(t)
	r := srv.Router()
	// 三顶层键齐全,但 dep.engines 缺 govulncheck
	body := `{"rules":{"enabled":false},"secret":{"enabled":true,"binary":"/opt/gitleaks"},"dep":{"enabled":true,"engines":{"npm":{"enabled":false}}}}`
	req := httptest.NewRequest(http.MethodPut, "/api/detectors/config", strings.NewReader(body))
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
	var resp map[string]map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"]["code"] != "invalid_config" {
		t.Errorf("error code = %q, want invalid_config; body = %s", resp["error"]["code"], w.Body.String())
	}
}
