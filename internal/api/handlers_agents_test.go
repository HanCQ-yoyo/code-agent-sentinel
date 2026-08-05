package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
)

// reqAgent 是 agents 路由测试辅助:发请求(可选 body),返回 recorder。
func reqAgent(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	r := s.Router()
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, path, nil)
	} else {
		b, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGetAgents(t *testing.T) {
	dir := t.TempDir()
	gin.SetMode(gin.TestMode)
	eng := configengine.NewEngine(dir, "")
	agents := configengine.DefaultAgents(dir, "")
	s := NewServer(eng, nil, config.DefaultConfig(), "tok", nil, agents, nil, nil)
	r := s.Router()
	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	var body struct {
		Agents []struct {
			ID          string `json:"id"`
			ScanEnabled bool   `json:"scan_enabled"`
		} `json:"agents"`
		Current string `json:"current"`
	}
	json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Agents) != 1 {
		t.Fatalf("agents 长度 = %d, 期望 1", len(body.Agents))
	}
	if body.Agents[0].ID != "claude-code" {
		t.Errorf("agents[0].ID = %q", body.Agents[0].ID)
	}
	if body.Current != "claude-code" {
		t.Errorf("current = %q, 期望 claude-code", body.Current)
	}
}

func TestPutAgentScanEnabled(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	// 注入 ScheduleRepo 使 ScanEnabled 可持久化
	s.SchedRepo = config.NewScheduleRepo(s.DB)
	s.Config.Agents = []config.AgentCfg{{ID: "claude-code", Enabled: true}}
	// 初始:scan_enabled 默认 true(SchedRepo 空)
	w := reqAgent(t, s, "GET", "/api/agents", nil)
	var body struct {
		Agents []struct {
			ID          string `json:"id"`
			ScanEnabled bool   `json:"scan_enabled"`
		} `json:"agents"`
	}
	json.NewDecoder(w.Body).Decode(&body)
	if len(body.Agents) == 0 || !body.Agents[0].ScanEnabled {
		t.Fatalf("GET 默认应 scan_enabled=true: %+v", body.Agents)
	}
	// 关闭
	w = reqAgent(t, s, "PUT", "/api/agents/claude-code", map[string]bool{"scan_enabled": false})
	if w.Code != http.StatusOK {
		t.Fatalf("PUT status: %d", w.Code)
	}
	// 验证 GET 展开为 false
	w = reqAgent(t, s, "GET", "/api/agents", nil)
	json.NewDecoder(w.Body).Decode(&body)
	if body.Agents[0].ScanEnabled {
		t.Error("关闭后 GET 应 scan_enabled=false")
	}
}

func TestPutAgentScanEnabled_Unknown(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	w := reqAgent(t, s, "PUT", "/api/agents/unknown", map[string]bool{"scan_enabled": false})
	if w.Code != http.StatusBadRequest {
		t.Errorf("未知 agent 应 400: got %d", w.Code)
	}
}

// TestGetAgentsMixedClaudeAndCodex 验证混合部署(claude-code + codex)下 /api/agents 返回两者且 Kind 正确。
func TestGetAgentsMixedClaudeAndCodex(t *testing.T) {
	dir := t.TempDir()
	gin.SetMode(gin.TestMode)
	eng := configengine.NewEngine(dir, "")
	agents := []configengine.Agent{
		{ID: "claude-code", Name: "Claude Code", Kind: "claude-code", RootDir: filepath.Join(dir, ".claude"), HomeDir: dir},
		{ID: "codex", Name: "Codex CLI", Kind: "codex", RootDir: filepath.Join(dir, ".codex"), HomeDir: dir},
	}
	s := NewServer(eng, nil, config.DefaultConfig(), "tok", nil, agents, nil, nil)
	w := reqAgent(t, s, "GET", "/api/agents", nil)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	var body struct {
		Agents []struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		} `json:"agents"`
	}
	json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Agents) != 2 {
		t.Fatalf("got %d agents, want 2", len(body.Agents))
	}
	got := map[string]string{}
	for _, a := range body.Agents {
		got[a.ID] = a.Kind
	}
	if got["claude-code"] != "claude-code" {
		t.Fatalf("claude-code Kind = %q, want claude-code", got["claude-code"])
	}
	if got["codex"] != "codex" {
		t.Fatalf("codex Kind = %q, want codex", got["codex"])
	}
}

// TestPutAgentScanEnabled_FallbackAgent 覆盖回退 agent 场景:用户没跑过 `sentinel setup`。
// ScanEnabled 持久化到 ScheduleRepo(而非 Config.Agents.ScanEnabled)。
func TestPutAgentScanEnabled_FallbackAgent(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.SchedRepo = config.NewScheduleRepo(s.DB)
	s.Config.Agents = nil
	// 初始 GET:回退 agent 默认 scan_enabled=true
	w := reqAgent(t, s, "GET", "/api/agents", nil)
	var body struct {
		Agents []struct {
			ID          string `json:"id"`
			ScanEnabled bool   `json:"scan_enabled"`
		} `json:"agents"`
	}
	json.NewDecoder(w.Body).Decode(&body)
	if len(body.Agents) == 0 || !body.Agents[0].ScanEnabled {
		t.Fatalf("回退 agent GET 默认应 scan_enabled=true: %+v", body.Agents)
	}
	// 关闭
	w = reqAgent(t, s, "PUT", "/api/agents/claude-code", map[string]bool{"scan_enabled": false})
	if w.Code != http.StatusOK {
		t.Fatalf("PUT status: %d %s", w.Code, w.Body)
	}
	// 验证 GET 展开为 false
	w = reqAgent(t, s, "GET", "/api/agents", nil)
	json.NewDecoder(w.Body).Decode(&body)
	if body.Agents[0].ScanEnabled {
		t.Error("回退 agent 关闭后 GET 应 scan_enabled=false(PUT 不应是 no-op)")
	}
}
