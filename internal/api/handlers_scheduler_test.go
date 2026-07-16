package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"code-agent-sentinel/internal/config"
	"code-agent-sentinel/internal/scheduler"
)

func newSchedulerTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.Scheduler = scheduler.New(0, func(context.Context) error { return nil })
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	return s
}

func reqScheduler(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
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

func TestGetSchedulerStatus(t *testing.T) {
	s := newSchedulerTestServer(t)
	w := reqScheduler(t, s, "GET", "/api/scheduler", nil)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	var st map[string]any
	json.Unmarshal(w.Body.Bytes(), &st)
	if st["enabled"] != false {
		t.Errorf("默认应 disabled,got %v", st["enabled"])
	}
}

func TestPutSchedulerEnablesAndPersists(t *testing.T) {
	s := newSchedulerTestServer(t)
	w := reqScheduler(t, s, "PUT", "/api/scheduler", map[string]any{"enabled": true, "interval": "1h"})
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	if !s.Scheduler.Status().Enabled {
		t.Error("应 enabled")
	}
	if s.Config.ScanEnabled != true || s.Config.ScanInterval != "1h" {
		t.Errorf("config 未落盘: enabled=%v interval=%q", s.Config.ScanEnabled, s.Config.ScanInterval)
	}
	// 落盘验证
	cfg, _ := config.Load(s.ConfigPath)
	if !cfg.ScanEnabled || cfg.ScanInterval != "1h" {
		t.Errorf("文件未落盘: %+v", cfg)
	}
	// 关闭
	w2 := reqScheduler(t, s, "PUT", "/api/scheduler", map[string]any{"enabled": false, "interval": "1h"})
	if w2.Code != 200 {
		t.Fatalf("关闭 got %d", w2.Code)
	}
	if s.Scheduler.Status().Enabled {
		t.Error("应 disabled")
	}
}

func TestPutSchedulerRejectsBadInterval(t *testing.T) {
	s := newSchedulerTestServer(t)
	w := reqScheduler(t, s, "PUT", "/api/scheduler", map[string]any{"enabled": true, "interval": "not-a-duration"})
	if w.Code != 400 {
		t.Fatalf("无效 interval 应 400,got %d: %s", w.Code, w.Body)
	}
}
