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
// Paused 是全局扫描总开关的内存闸门:为 true 时所有任务的 tick 跳过 run
// (但不改各 schedule 的 enabled/interval,Status 仍如实报告)。供 PUT /api/settings
// 的 scan_enabled 总开关使用——总开关是"门",per-agent schedule.enabled 是"哪些过门"。
type Manager struct {
	mu      sync.Mutex
	runners map[string]*Scheduler // agentID → Scheduler
	makeRun func(agentID string) func(context.Context) error
	paused  bool
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
			sch.Reconfigure(enabled, d) // 改间隔/启停;不重建 run(Scheduler 持有的 run 已是包装版,闸门始终读最新 m.paused)
			continue
		}
		sch := New(d, m.wrapRun(id))
		m.runners[id] = sch
		if enabled {
			sch.Start()
		}
	}
}

// wrapRun 把 run 回调包一层:Manager.paused 时跳过执行。
// 在 Scheduler 创建时包装一次,Reconfigure 不重建 run——闸门始终读最新 m.paused,
// 因此 reconfigure 只改 interval/enabled 即可,不需要重新包装。
func (m *Manager) wrapRun(agentID string) func(context.Context) error {
	inner := m.makeRun(agentID)
	return func(ctx context.Context) error {
		m.mu.Lock()
		paused := m.paused
		m.mu.Unlock()
		if paused {
			return nil // 总开关关:跳过本次 tick
		}
		return inner(ctx)
	}
}

// SetPaused 设置全局暂停闸门。true=所有任务 tick 跳过 run(不改 schedule.enabled)。
func (m *Manager) SetPaused(paused bool) {
	m.mu.Lock()
	m.paused = paused
	m.mu.Unlock()
}

// Paused 返回当前闸门状态。
func (m *Manager) Paused() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.paused
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
