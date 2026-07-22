package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/configengine"
)

// newTwoAgentTestServer 构造注入 2 个 agent(root 不同)的测试 Server,供 agent 化读路径测试。
// agentA root = home/.claude;agentB root = home/.claude-b(需有 settings.json 触发不同发现)。
func newTwoAgentTestServer(t *testing.T, home string) *Server {
	t.Helper()
	writeFile(t, filepath.Join(home, ".claude", "settings.json"), `{"permissions":{"allow":["Bash(*)"]}}`)
	writeFile(t, filepath.Join(home, ".claude-b", "settings.json"), `{"model":"opus"}`)
	agents := []configengine.Agent{
		{ID: "a", Name: "Agent A", RootDir: filepath.Join(home, ".claude"), ClaudeJSON: filepath.Join(home, ".claude.json"), HomeDir: home},
		{ID: "b", Name: "Agent B", RootDir: filepath.Join(home, ".claude-b"), ClaudeJSON: filepath.Join(home, ".claude-b.json"), HomeDir: home},
	}
	eng := configengine.NewEngineFromAgent(agents[0])
	s := newTestServerWithAgents(t, eng, agents)
	return s
}

// TestPostScanRejectsUnknownAgent 验证 postScan 对未知 ?agent= 返回 400 unknown_agent,
// 不再静默回退首 agent(消除误用隐患——拼写错的 agent ID 应显式报错)。
func TestPostScanRejectsUnknownAgent(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	w := reqScan(t, s, "POST", "/api/scan?agent=does-not-exist", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("未知 agent 应 400: got %d %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "unknown_agent") {
		t.Errorf("应含 unknown_agent code: %s", w.Body)
	}
}

// TestPostScanEmptyAgentFallsBack 验证空 agent(无 ?agent= query)回退首 agent,
// 向后兼容 sentinel scan 无 --agent、scheduler 内部调用。
func TestPostScanEmptyAgentFallsBack(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	spy := &spyRunner{}
	s.Runner = spy
	w := reqScan(t, s, "POST", "/api/scan", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("空 agent 应回退首 agent: got %d %s", w.Code, w.Body)
	}
}

// TestEngineForQueryFallsBackToFirstAgent 验证 engineForQuery 在无 ?agent= 且无 SelectedAgentID 时
// 回退首 agent。用真实 Runner(newTestServer 内置 scan.NewRunner,EngineFor 池化非 nil),
// 不用 spyRunner(其 EngineFor 返回 nil,无法验证 eng != nil)。
//
// 注:engineForQuery 接受 *gin.Context(与 handler 签名一致),测试用 gin.CreateTestContext
// 构造上下文后挂上 httptest.NewRequest 的 URL,使 c.Query("agent") 可读 query string。
func TestEngineForQueryFallsBackToFirstAgent(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.SelectedAgentID = "" // 强制走首 agent 回退
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/dashboard", nil)
	eng, id, err := s.engineForQuery(c)
	if err != nil {
		t.Fatalf("不应错: %v", err)
	}
	if eng == nil {
		t.Fatal("Engine 不应为 nil")
	}
	if id == "" {
		t.Error("agentID 不应为空(应回退首 agent)")
	}
}

// TestEngineForQueryUnknownAgentErrors 验证 ?agent= 未知 ID 时返回错误(调用方报 400)。
func TestEngineForQueryUnknownAgentErrors(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/dashboard?agent=zzz", nil)
	eng, id, err := s.engineForQuery(c)
	if err == nil {
		t.Fatal("未知 agent 应返回错误")
	}
	if eng != nil {
		t.Errorf("未知 agent 不应返回 Engine: got %v", eng)
	}
	if id != "" {
		t.Errorf("未知 agent 不应返回 agentID: got %q", id)
	}
}

// TestEngineForQueryHonorsAgentQuery 验证 ?agent= 优先于 SelectedAgentID。
func TestEngineForQueryHonorsAgentQuery(t *testing.T) {
	dir := t.TempDir()
	// 用两 agent fixture,使 ?agent=b 能解析到非首 agent 的 Engine
	s := newTwoAgentTestServer(t, dir)
	s.SelectedAgentID = "a"
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/dashboard?agent=b", nil)
	eng, id, err := s.engineForQuery(c)
	if err != nil {
		t.Fatalf("不应错: %v", err)
	}
	if eng == nil {
		t.Fatal("Engine 不应为 nil")
	}
	if id != "b" {
		t.Errorf("agentID 应为 b: got %q", id)
	}
	// Engine 的 RootDir 应指向 agent B 的根(home/.claude-b)
	want := filepath.Join(dir, ".claude-b")
	if eng.ClaudeDir != want {
		t.Errorf("Engine.ClaudeDir 应为 %s: got %s", want, eng.ClaudeDir)
	}
}

// TestAgentExists 验证 agentExists 的基本行为。
func TestAgentExists(t *testing.T) {
	dir := t.TempDir()
	s := newTwoAgentTestServer(t, dir)
	if !s.agentExists("a") {
		t.Error("agentExists(a) 应为 true")
	}
	if !s.agentExists("b") {
		t.Error("agentExists(b) 应为 true")
	}
	if s.agentExists("zzz") {
		t.Error("agentExists(zzz) 应为 false")
	}
	// 空串不在 s.Agents 中(agentExists 严格匹配 ID)
	if s.agentExists("") {
		t.Error("agentExists(\"\") 应为 false(空串不是合法 agent ID)")
	}
}
