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

// newSchedulerTestServer 构造带真实 ScheduleManager 的测试 server(替代旧版
// 注入 dead s.Scheduler)。预设一个 AgentID==SelectedAgentID、enabled=false、
// interval=1h 的任务,使 /api/scheduler 的 deprecated 转发路径有任务可改。
func newSchedulerTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ScheduleManager = scheduler.NewManager(func(string) func(context.Context) error {
		return func(context.Context) error { return nil }
	})
	s.Config.Schedules = []config.ScheduleCfg{{AgentID: s.SelectedAgentID, Enabled: false, Interval: "1h"}}
	s.ScheduleManager.Apply(s.Config.Schedules)
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
	// Task 3:断言改为查 ScheduleManager.Status() 中 AgentID==SelectedAgentID 的任务
	//(旧 s.Scheduler.Status() 已随字段删除)。
	st := s.ScheduleManager.Status()
	var found bool
	for _, x := range st {
		if x.AgentID == s.SelectedAgentID {
			if !x.Enabled {
				t.Error("schedule 任务应 enabled")
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("未找到 %s 的 schedule 任务", s.SelectedAgentID)
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
	for _, x := range s.ScheduleManager.Status() {
		if x.AgentID == s.SelectedAgentID && x.Enabled {
			t.Error("关闭后 schedule 任务应 disabled")
		}
	}
}

func TestPutSchedulerRejectsBadInterval(t *testing.T) {
	s := newSchedulerTestServer(t)
	w := reqScheduler(t, s, "PUT", "/api/scheduler", map[string]any{"enabled": true, "interval": "not-a-duration"})
	if w.Code != 400 {
		t.Fatalf("无效 interval 应 400,got %d: %s", w.Code, w.Body)
	}
}

// newNilSchedulerTestServer 构造 ScheduleManager == nil 的 server,测试 /api/scheduler
// 的退化路径(schedulerStatusResponse 在无 Manager 时基于 config 构造)。
// Task 3 后:s.Scheduler 字段已删,nil-safety 的语义改为 ScheduleManager==nil 退化。
func newNilSchedulerTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s := newTestServer(t, dir)
	// newTestServer 不设 ScheduleManager(默认 nil),这里显式置 nil 并设 ConfigPath。
	s.ScheduleManager = nil
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	return s
}

// TestSchedulerNilSafeGetAndPut 覆盖 Important finding:ScheduleManager == nil 时
// GET 与 PUT /api/scheduler 都必须返回 200(基于 config 退化构造响应),不得 panic/500。
func TestSchedulerNilSafeGetAndPut(t *testing.T) {
	s := newNilSchedulerTestServer(t)
	// GET:nil ScheduleManager 不 panic,返回 200 + 基于 config 的退化状态
	w := reqScheduler(t, s, "GET", "/api/scheduler", nil)
	if w.Code != 200 {
		t.Fatalf("GET nil ScheduleManager 应 200(退化),got %d: %s", w.Code, w.Body)
	}
	var st map[string]any
	json.Unmarshal(w.Body.Bytes(), &st)
	if st["enabled"] != false {
		t.Errorf("默认 ScanEnabled=false,退化应 disabled,got %v", st["enabled"])
	}
	if st["interval"] != "0s" {
		t.Errorf("空 ScanInterval 退化应 \"0s\",got %v", st["interval"])
	}
	// PUT:nil ScheduleManager 不 panic,返回 200,config 落盘
	w2 := reqScheduler(t, s, "PUT", "/api/scheduler", map[string]any{"enabled": true, "interval": "1h"})
	if w2.Code != 200 {
		t.Fatalf("PUT nil ScheduleManager 应 200(退化),got %d: %s", w2.Code, w2.Body)
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
	s.ScheduleManager = nil
	s.ConfigPath = "" // 显式:跳过 Save 路径
	w := reqScheduler(t, s, "PUT", "/api/scheduler", map[string]any{"enabled": true, "interval": "30m"})
	if w.Code != 200 {
		t.Fatalf("PUT nil ScheduleManager + 空 ConfigPath 应 200,got %d: %s", w.Code, w.Body)
	}
	if !s.Config.ScanEnabled || s.Config.ScanInterval != "30m" {
		t.Errorf("内存 config 应更新: enabled=%v interval=%q", s.Config.ScanEnabled, s.Config.ScanInterval)
	}
}

func TestPutSchedulerDeprecatedWritesSchedules(t *testing.T) {
	s := newSchedulesTestServer(t) // 带 ScheduleManager
	// 旧端点 PUT 设 interval=1h enabled=true
	w := reqScheduler(t, s, "PUT", "/api/scheduler", map[string]any{"enabled": true, "interval": "1h"})
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	cfg, _ := config.Load(s.ConfigPath)
	if len(cfg.Schedules) != 1 || cfg.Schedules[0].Interval != "1h" {
		t.Errorf("旧端点 PUT 应写 schedules: %+v", cfg.Schedules)
	}
}

func TestGetSchedulerDeprecatedReadsSchedules(t *testing.T) {
	s := newSchedulesTestServer(t)
	s.Config.Schedules = []config.ScheduleCfg{{AgentID: "claude-code", Enabled: true, Interval: "30m"}}
	s.applySchedules()
	w := reqScheduler(t, s, "GET", "/api/scheduler", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var resp schedulerResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Enabled || resp.Interval != "30m" {
		t.Errorf("旧 GET 应读 schedules 首项: %+v", resp)
	}
}
