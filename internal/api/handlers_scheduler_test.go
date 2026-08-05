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

// newSchedulerTestServer 构造带真实 ScheduleManager 的测试 server。
// 预设一个 AgentID==SelectedAgentID、enabled=true、interval=1h 的调度任务。
func newSchedulerTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ScheduleManager = scheduler.NewManager(func(string) func(context.Context) error {
		return func(context.Context) error { return nil }
	})
	// 注入 ScheduleRepo 并预置一条调度
	s.SchedRepo = config.NewScheduleRepo(s.DB)
	_ = s.SchedRepo.Upsert(s.SelectedAgentID, true, "1h")
	s.applySchedules()
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
	if st["enabled"] != true {
		t.Errorf("预设 enabled=true,got %v", st["enabled"])
	}
}

func TestPutSchedulerEnablesAndPersists(t *testing.T) {
	s := newSchedulerTestServer(t)
	w := reqScheduler(t, s, "PUT", "/api/scheduler", map[string]any{"enabled": true, "interval": "1h"})
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	// 断言 ScheduleManager.Status() 中 AgentID==SelectedAgentID 的任务
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
	// 验证经 ScheduleRepo 持久化
	scs, err := s.SchedRepo.List()
	if err != nil || len(scs) == 0 {
		t.Fatalf("ScheduleRepo 为空, err=%v", err)
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

// newNilSchedulerTestServer 构造 ScheduleManager == nil 的 server。
func newNilSchedulerTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ScheduleManager = nil
	s.ConfigPath = filepath.Join(dir, "config.yaml")
	return s
}

// TestSchedulerNilSafeGetAndPut 覆盖 Important finding:ScheduleManager == nil 时
// GET 与 PUT /api/scheduler 都必须返回 200(退化构造响应),不得 panic/500。
func TestSchedulerNilSafeGetAndPut(t *testing.T) {
	s := newNilSchedulerTestServer(t)
	// GET:nil ScheduleManager 不 panic
	w := reqScheduler(t, s, "GET", "/api/scheduler", nil)
	if w.Code != 200 {
		t.Fatalf("GET nil ScheduleManager 应 200(退化),got %d: %s", w.Code, w.Body)
	}
	var st map[string]any
	json.Unmarshal(w.Body.Bytes(), &st)
	// 退化返回 false + "0s"(SchedRepo 为 nil)
	if st["enabled"] != false {
		t.Errorf("退化应 disabled,got %v", st["enabled"])
	}
	if st["interval"] != "0s" {
		t.Errorf("退化 interval 应 \"0s\",got %v", st["interval"])
	}
	// PUT:nil ScheduleManager 不 panic
	w2 := reqScheduler(t, s, "PUT", "/api/scheduler", map[string]any{"enabled": true, "interval": "1h"})
	if w2.Code != 200 {
		t.Fatalf("PUT nil ScheduleManager 应 200(退化),got %d: %s", w2.Code, w2.Body)
	}
	var st2 map[string]any
	json.Unmarshal(w2.Body.Bytes(), &st2)
	// 退化时 PUT 不持久化(SchedRepo nil),响应仍反映退化状态
	if st2["enabled"] != false {
		t.Errorf("退化 PUT 后应仍 disabled(SchedRepo nil),got %v", st2["enabled"])
	}
}

// TestSchedulerNilSafeNoConfigPath 覆盖 Minor finding:s.ConfigPath == "" 时
// PUT /api/scheduler 跳过 Save(不 panic),仍返回 200。
func TestSchedulerNilSafeNoConfigPath(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, dir)
	s.ScheduleManager = nil
	s.ConfigPath = ""
	w := reqScheduler(t, s, "PUT", "/api/scheduler", map[string]any{"enabled": true, "interval": "30m"})
	if w.Code != 200 {
		t.Fatalf("PUT nil ScheduleManager + 空 ConfigPath 应 200,got %d: %s", w.Code, w.Body)
	}
}

func TestPutSchedulerDeprecatedWritesSchedules(t *testing.T) {
	s := newSchedulesTestServer(t)
	// 旧端点 PUT
	w := reqScheduler(t, s, "PUT", "/api/scheduler", map[string]any{"enabled": true, "interval": "1h"})
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	// 验证经 ScheduleRepo 持久化
	scs, _ := s.SchedRepo.List()
	if len(scs) != 1 || scs[0].Interval != "1h" {
		t.Errorf("旧端点 PUT 应写 schedules: %+v", scs)
	}
}

func TestGetSchedulerDeprecatedReadsSchedules(t *testing.T) {
	s := newSchedulesTestServer(t)
	// 预置调度
	_ = s.SchedRepo.Upsert("claude-code", true, "30m")
	s.applySchedules()
	w := reqScheduler(t, s, "GET", "/api/scheduler", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var resp schedulerResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Enabled || resp.Interval != "30m" {
		t.Errorf("旧 GET 应读 schedules: %+v", resp)
	}
}
