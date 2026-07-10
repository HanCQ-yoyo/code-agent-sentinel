package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/configengine"
)

func TestGetAgents(t *testing.T) {
	dir := t.TempDir()
	gin.SetMode(gin.TestMode)
	eng := configengine.NewEngine(dir)
	agents := configengine.DefaultAgents(dir)
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
		Agents  []configengine.Agent `json:"agents"`
		Current string               `json:"current"`
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
