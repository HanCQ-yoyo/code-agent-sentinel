package scheduler

import (
	"context"
	"sync"
	"time"

	"code-agent-sentinel/internal/config"
)

// ScheduleStatus 是单个 agent 定时任务的状态(GET /api/schedules 用)。
type ScheduleStatus struct {
	AgentID  string        `json:"agent_id"`
	Enabled  bool          `json:"enabled"`
	Interval time.Duration `json:"interval"`
	LastRun  time.Time     `json:"last_run"`
	NextRun  time.Time     `json:"next_run"`
}

// Manager 管理多个 Scheduler(每 agent 一个),按 config.ScheduleCfg 增量同步。
type Manager struct {
	mu      sync.Mutex
	runners map[string]*Scheduler // agentID → Scheduler
	makeRun func(agentID string) func(context.Context) error
}

// NewManager 构造 Manager。makeRun 工厂按 agentID 造 run 回调。
func NewManager(makeRun func(agentID string) func(context.Context) error) *Manager {
	return &Manager{runners: map[string]*Scheduler{}, makeRun: makeRun}
}

// Apply 增量同步:新增的 Start,删的 Stop,改间隔/启停的 Reconfigure,未变的不动。
// disabled 或 interval<=0 的任务不启动(但保留在 runners 里记录 Status)。
func (m *Manager) Apply(schedules []config.ScheduleCfg) {
	m.mu.Lock()
	defer m.mu.Unlock()
	want := map[string]config.ScheduleCfg{}
	for _, s := range schedules {
		want[s.AgentID] = s
	}
	// 删除不再要的
	for id, sch := range m.runners {
		if _, ok := want[id]; !ok {
			sch.Stop()
			delete(m.runners, id)
		}
	}
	// 新增 / 更新
	for id, s := range want {
		d, _ := time.ParseDuration(s.Interval)
		enabled := s.Enabled && d > 0
		if sch, ok := m.runners[id]; ok {
			sch.Reconfigure(enabled, d) // 改间隔/启停
			continue
		}
		sch := New(d, m.makeRun(id))
		m.runners[id] = sch
		if enabled {
			sch.Start()
		}
	}
}

// Status 返回所有任务的当前状态。
func (m *Manager) Status() []ScheduleStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ScheduleStatus, 0, len(m.runners))
	for id, sch := range m.runners {
		st := sch.Status()
		out = append(out, ScheduleStatus{
			AgentID: id, Enabled: st.Enabled, Interval: st.Interval, LastRun: st.LastRun, NextRun: st.NextRun,
		})
	}
	return out
}

// Stop 停止所有任务。可重复调用(main defer)。
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sch := range m.runners {
		sch.Stop()
	}
	m.runners = map[string]*Scheduler{}
}
