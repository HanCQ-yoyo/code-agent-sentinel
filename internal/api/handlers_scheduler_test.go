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

// newNilSchedulerTestServer 构造 s.Scheduler == nil 的 server,测试优雅降级。
// newTestServer 不设置 Scheduler(默认 nil),这里显式置 nil 并设 ConfigPath。
func newNilSchedulerTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.Scheduler = nil
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	return s
}

// TestSchedulerNilSafeGetAndPut 覆盖 Important finding:s.Scheduler == nil 时
// GET 与 PUT /api/scheduler 都必须返回 200(基于 config 退化构造响应),不得 panic/500。
func TestSchedulerNilSafeGetAndPut(t *testing.T) {
	s := newNilSchedulerTestServer(t)
	// GET:nil Scheduler 不 panic,返回 200 + 基于 config 的退化状态
	w := reqScheduler(t, s, "GET", "/api/scheduler", nil)
	if w.Code != 200 {
		t.Fatalf("GET nil Scheduler 应 200(退化),got %d: %s", w.Code, w.Body)
	}
	var st map[string]any
	json.Unmarshal(w.Body.Bytes(), &st)
	if st["enabled"] != false {
		t.Errorf("默认 ScanEnabled=false,退化应 disabled,got %v", st["enabled"])
	}
	if st["interval"] != "0s" {
		t.Errorf("空 ScanInterval 退化应 \"0s\",got %v", st["interval"])
	}
	// PUT:nil Scheduler 不 panic,返回 200,config 落盘
	w2 := reqScheduler(t, s, "PUT", "/api/scheduler", map[string]any{"enabled": true, "interval": "1h"})
	if w2.Code != 200 {
		t.Fatalf("PUT nil Scheduler 应 200(退化),got %d: %s", w2.Code, w2.Body)
	}
	var st2 map[string]any
	json.Unmarshal(w2.Body.Bytes(), &st2)
	if st2["enabled"] != true {
		t.Errorf("PUT 后应 enabled,got %v", st2["enabled"])
	}
	if st2["interval"] != "1h" {
		t.Errorf("退化 interval 应直接用 ScanInterval 字符串 \"1h\",got %v", st2["interval"])
	}
	if !s.Config.ScanEnabled || s.Config.ScanInterval != "1h" {
		t.Errorf("config 未落盘: enabled=%v interval=%q", s.Config.ScanEnabled, s.Config.ScanInterval)
	}
}

// TestSchedulerNilSafeNoConfigPath 覆盖 Minor finding:s.ConfigPath == "" 时
// PUT /api/scheduler 跳过 Save(不 panic),仍返回 200 退化状态。
func TestSchedulerNilSafeNoConfigPath(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.Scheduler = nil
	s.ConfigPath = "" // 显式:跳过 Save 路径
	w := reqScheduler(t, s, "PUT", "/api/scheduler", map[string]any{"enabled": true, "interval": "30m"})
	if w.Code != 200 {
		t.Fatalf("PUT nil Scheduler + 空 ConfigPath 应 200,got %d: %s", w.Code, w.Body)
	}
	if !s.Config.ScanEnabled || s.Config.ScanInterval != "30m" {
		t.Errorf("内存 config 应更新: enabled=%v interval=%q", s.Config.ScanEnabled, s.Config.ScanInterval)
	}
}
