package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"code-agent-sentinel/internal/config"
)

func TestManagerApplyStartsTasksPerAgent(t *testing.T) {
	var n int32
	mk := func(agentID string) func(context.Context) error {
		return func(context.Context) error { atomic.AddInt32(&n, 1); return nil }
	}
	m := NewManager(mk)
	m.Apply([]config.ScheduleCfg{
		{AgentID: "a", Enabled: true, Interval: "50ms"},
		{AgentID: "b", Enabled: true, Interval: "50ms"},
	})
	time.Sleep(200 * time.Millisecond)
	m.Stop()
	got := atomic.LoadInt32(&n)
	if got < 2 {
		t.Fatalf("两个任务都应触发,各至少 1 次: got %d", got)
	}
}

func TestManagerApplyStopsRemovedTask(t *testing.T) {
	var n int32
	mk := func(agentID string) func(context.Context) error {
		return func(context.Context) error { atomic.AddInt32(&n, 1); return nil }
	}
	m := NewManager(mk)
	m.Apply([]config.ScheduleCfg{{AgentID: "a", Enabled: true, Interval: "50ms"}})
	time.Sleep(150 * time.Millisecond)
	before := atomic.LoadInt32(&n)
	m.Apply(nil) // 移除全部
	time.Sleep(150 * time.Millisecond)
	after := atomic.LoadInt32(&n)
	if after > before {
		t.Errorf("移除后不应再触发: before=%d after=%d", before, after)
	}
	m.Stop()
}

func TestManagerApplyReconfiguresChangedInterval(t *testing.T) {
	var n int32
	mk := func(agentID string) func(context.Context) error {
		return func(context.Context) error { atomic.AddInt32(&n, 1); return nil }
	}
	m := NewManager(mk)
	m.Apply([]config.ScheduleCfg{{AgentID: "a", Enabled: true, Interval: "50ms"}})
	m.Apply([]config.ScheduleCfg{{AgentID: "a", Enabled: true, Interval: "200ms"}}) // 改间隔
	st := m.Status()
	if len(st) != 1 || st[0].Interval != 200*time.Millisecond {
		t.Fatalf("应 Reconfigure 到新间隔: %+v", st)
	}
	m.Stop()
}

func TestManagerStatusAggregatesAll(t *testing.T) {
	m := NewManager(func(string) func(context.Context) error {
		return func(context.Context) error { return nil }
	})
	m.Apply([]config.ScheduleCfg{
		{AgentID: "a", Enabled: true, Interval: "1m"},
		{AgentID: "b", Enabled: false, Interval: "1m"},
	})
	st := m.Status()
	if len(st) != 2 {
		t.Fatalf("Status 应返回全部任务: got %d", len(st))
	}
	m.Stop()
}

func TestManagerApplyDisabledDoesNotStart(t *testing.T) {
	var n int32
	m := NewManager(func(string) func(context.Context) error {
		return func(context.Context) error { atomic.AddInt32(&n, 1); return nil }
	})
	m.Apply([]config.ScheduleCfg{{AgentID: "a", Enabled: false, Interval: "50ms"}})
	time.Sleep(150 * time.Millisecond)
	if atomic.LoadInt32(&n) != 0 {
		t.Errorf("disabled 任务不应触发: got %d", n)
	}
	m.Stop()
}

func TestManagerPausedSuppressesTicks(t *testing.T) {
	var n int32
	mk := func(agentID string) func(context.Context) error {
		return func(context.Context) error { atomic.AddInt32(&n, 1); return nil }
	}
	m := NewManager(mk)
	m.Apply([]config.ScheduleCfg{{AgentID: "a", Enabled: true, Interval: "50ms"}})
	time.Sleep(120 * time.Millisecond) // 至少跑 1-2 次
	before := atomic.LoadInt32(&n)
	m.SetPaused(true) // 总开关关:所有任务暂停
	// Paused 只闸 tick,不改 schedule 配置——Status 仍应报告 enabled=true, interval=50ms。
	if st := m.Status(); len(st) != 1 || !st[0].Enabled || st[0].Interval != 50*time.Millisecond {
		t.Errorf("Paused 不应改 schedule 配置: %+v", st)
	}
	time.Sleep(200 * time.Millisecond)
	during := atomic.LoadInt32(&n)
	if during > before {
		t.Errorf("Paused 后不应再触发: before=%d during=%d", before, during)
	}
	m.SetPaused(false) // 恢复:重新跑
	time.Sleep(200 * time.Millisecond)
	after := atomic.LoadInt32(&n)
	if after <= during {
		t.Errorf("解除 Paused 后应恢复触发: during=%d after=%d", during, after)
	}
	m.Stop()
}
