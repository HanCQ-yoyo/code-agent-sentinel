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
// 与 reqScan 同模式,区别在不注入 spyRunner。
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
	s := NewServer(eng, nil, config.DefaultConfig(), "tok", nil, agents, nil)
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
	// 无 AgentCfg 时 ScanEnabled 应默认 true
	if !body.Agents[0].ScanEnabled {
		t.Error("GET 默认应 scan_enabled=true")
	}
}

func TestPutAgentScanEnabled(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	// 预置 AgentCfg 使 PUT 能找到并更新对应项
	s.Config.Agents = []config.AgentCfg{{ID: "claude-code", Enabled: true}}
	// 初始:scan_enabled 默认 true(nil 展开)
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
// 模拟 main.go 经 AgentsFromSpecs 填充后的 s.Agents,直接构造混合切片传入 NewServer。
func TestGetAgentsMixedClaudeAndCodex(t *testing.T) {
	dir := t.TempDir()
	gin.SetMode(gin.TestMode)
	eng := configengine.NewEngine(dir, "")
	// 直接构造混合 agents(模拟 main.go 经 AgentsFromSpecs 填充后的结果)
	agents := []configengine.Agent{
		{ID: "claude-code", Name: "Claude Code", Kind: "claude-code", RootDir: filepath.Join(dir, ".claude"), HomeDir: dir},
		{ID: "codex", Name: "Codex CLI", Kind: "codex", RootDir: filepath.Join(dir, ".codex"), HomeDir: dir},
	}
	s := NewServer(eng, nil, config.DefaultConfig(), "tok", nil, agents, nil)
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

// TestPutAgentScanEnabled_FallbackAgent 覆盖回退 agent 场景:用户没跑过 `sentinel setup`,
// config.yaml 是 agents: [] → ResolveAgents 走回退路径合成 claude-code,但 Config.Agents 仍空。
// 此时 s.Agents 含 claude-code(agentExists=true,开关显示),但 s.Config.Agents 为空。
// PUT scan_enabled 应能切换并持久化(否则开关弹回「开」,对回退用户完全失效)。
func TestPutAgentScanEnabled_FallbackAgent(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	// 模拟回退:s.Agents 含 claude-code(newTestServer 已置),但 Config.Agents 为空。
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
	// 验证 GET 展开为 false(若 PUT 是 no-op,GET 仍返回默认 true → 测试失败)
	w = reqAgent(t, s, "GET", "/api/agents", nil)
	json.NewDecoder(w.Body).Decode(&body)
	if body.Agents[0].ScanEnabled {
		t.Error("回退 agent 关闭后 GET 应 scan_enabled=false(PUT 不应是 no-op)")
	}
	// 验证已落盘:Config.Agents 现应含该 agent 且 ScanEnabled=false
	if len(s.Config.Agents) != 1 || s.Config.Agents[0].ID != "claude-code" {
		t.Fatalf("PUT 后 Config.Agents 应含 claude-code: %+v", s.Config.Agents)
	}
	if s.Config.Agents[0].ScanEnabled == nil || *s.Config.Agents[0].ScanEnabled != false {
		t.Errorf("Config.Agents[0].ScanEnabled 应为 *false: %v", s.Config.Agents[0].ScanEnabled)
	}
}
