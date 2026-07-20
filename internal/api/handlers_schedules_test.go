package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/scheduler"
)

func newSchedulesTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ScheduleManager = scheduler.NewManager(func(string) func(context.Context) error {
		return func(context.Context) error { return nil }
	})
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	s.Config.Agents = []config.AgentCfg{{ID: "claude-code", Enabled: true}}
	return s
}

func TestGetSchedulesEmpty(t *testing.T) {
	s := newSchedulesTestServer(t)
	w := reqSchedules(t, s, "GET", "/api/schedules", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var resp struct{ Schedules []scheduler.ScheduleStatus }
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Schedules) != 0 {
		t.Errorf("空应返回 0 任务: got %d", len(resp.Schedules))
	}
}

func TestPostScheduleCreatesTask(t *testing.T) {
	s := newSchedulesTestServer(t)
	w := reqSchedules(t, s, "POST", "/api/schedules", map[string]any{
		"agent_id": "claude-code", "enabled": true, "interval": "1h",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	// 落盘校验
	cfg, _ := config.Load(s.ConfigPath)
	if len(cfg.Schedules) != 1 || cfg.Schedules[0].AgentID != "claude-code" {
		t.Errorf("应落盘 1 任务: %+v", cfg.Schedules)
	}
	// Status 能查到
	w2 := reqSchedules(t, s, "GET", "/api/schedules", nil)
	var resp struct{ Schedules []scheduler.ScheduleStatus }
	json.Unmarshal(w2.Body.Bytes(), &resp)
	if len(resp.Schedules) != 1 {
		t.Errorf("Status 应有 1 任务: got %d", len(resp.Schedules))
	}
}

func TestPostScheduleRejectsDuplicateAgent(t *testing.T) {
	s := newSchedulesTestServer(t)
	reqSchedules(t, s, "POST", "/api/schedules", map[string]any{"agent_id": "claude-code", "interval": "1h"})
	w := reqSchedules(t, s, "POST", "/api/schedules", map[string]any{"agent_id": "claude-code", "interval": "30m"})
	if w.Code != http.StatusConflict {
		t.Errorf("重复 agent_id 应 409: got %d", w.Code)
	}
}

func TestPostScheduleRejectsBadInterval(t *testing.T) {
	s := newSchedulesTestServer(t)
	w := reqSchedules(t, s, "POST", "/api/schedules", map[string]any{"agent_id": "claude-code", "interval": "notaduration"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("坏 interval 应 400: got %d", w.Code)
	}
}

func TestPutScheduleUpdatesInterval(t *testing.T) {
	s := newSchedulesTestServer(t)
	reqSchedules(t, s, "POST", "/api/schedules", map[string]any{"agent_id": "claude-code", "interval": "1h"})
	w := reqSchedules(t, s, "PUT", "/api/schedules/claude-code", map[string]any{"enabled": false, "interval": "2h"})
	if w.Code != http.StatusOK {
		t.Fatalf("put: %d %s", w.Code, w.Body.String())
	}
	cfg, _ := config.Load(s.ConfigPath)
	if cfg.Schedules[0].Interval != "2h" || cfg.Schedules[0].Enabled {
		t.Errorf("应更新: %+v", cfg.Schedules[0])
	}
}

func TestDeleteScheduleRemovesTask(t *testing.T) {
	s := newSchedulesTestServer(t)
	reqSchedules(t, s, "POST", "/api/schedules", map[string]any{"agent_id": "claude-code", "interval": "1h"})
	w := reqSchedules(t, s, "DELETE", "/api/schedules/claude-code", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: %d", w.Code)
	}
	cfg, _ := config.Load(s.ConfigPath)
	if len(cfg.Schedules) != 0 {
		t.Errorf("应删除: %+v", cfg.Schedules)
	}
}

func TestDeleteScheduleNotFound(t *testing.T) {
	s := newSchedulesTestServer(t)
	w := reqSchedules(t, s, "DELETE", "/api/schedules/nope", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("不存在应 404: got %d", w.Code)
	}
}

var _ = time.Second // 防止 time 未用 import(若测试未直接用)

func reqSchedules(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
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
