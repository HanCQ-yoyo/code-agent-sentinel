package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerTicksAndStops(t *testing.T) {
	var n int32
	run := func(context.Context) error { atomic.AddInt32(&n, 1); return nil }
	s := New(50*time.Millisecond, run)
	s.Start()
	time.Sleep(200 * time.Millisecond) // 50ms 间隔,~3-4 次 tick
	s.Stop()
	got := atomic.LoadInt32(&n)
	if got < 2 {
		t.Fatalf("应至少触发 2 次,got %d", got)
	}
	// Stop 后不再触发
	after := got
	time.Sleep(150 * time.Millisecond)
	if atomic.LoadInt32(&n) > after {
		t.Errorf("Stop 后不应再触发: before=%d after=%d", after, atomic.LoadInt32(&n))
	}
}

func TestSchedulerInFlightSkip(t *testing.T) {
	var n int32
	// run 持续 200ms(> 50ms 间隔),inFlight 期间 tick 应跳过
	run := func(ctx context.Context) error {
		atomic.AddInt32(&n, 1)
		select {
		case <-time.After(200 * time.Millisecond):
		case <-ctx.Done():
		}
		return nil
	}
	s := New(50*time.Millisecond, run)
	s.Start()
	time.Sleep(300 * time.Millisecond)
	s.Stop()
	// 300ms 内:首次 run 占 200ms,期间 3-4 次 tick 被跳过,200ms 后第二次 run 开始
	got := atomic.LoadInt32(&n)
	if got > 2 {
		t.Errorf("inFlight 期间应跳过并发 run,应 ≤2 次,got %d", got)
	}
}

func TestSchedulerStatus(t *testing.T) {
	run := func(context.Context) error { return nil }
	s := New(100*time.Millisecond, run)
	st := s.Status()
	if st.Enabled {
		t.Error("未 Start 应 Enabled=false")
	}
	s.Start()
	st = s.Status()
	if !st.Enabled || st.Interval != 100*time.Millisecond {
		t.Errorf("Start 后 Status: %+v", st)
	}
	s.Stop()
	if s.Status().Enabled {
		t.Error("Stop 后应 Enabled=false")
	}
}

func TestSchedulerReconfigure(t *testing.T) {
	var n int32
	run := func(context.Context) error { atomic.AddInt32(&n, 1); return nil }
	s := New(50*time.Millisecond, run)
	s.Start()
	time.Sleep(120 * time.Millisecond)
	s.Reconfigure(false, 0) // 关闭
	time.Sleep(120 * time.Millisecond)
	after := atomic.LoadInt32(&n)
	if s.Status().Enabled {
		t.Error("Reconfigure(false) 后应 disabled")
	}
	time.Sleep(120 * time.Millisecond)
	if atomic.LoadInt32(&n) > after {
		t.Errorf("Reconfigure(false) 后不应再触发")
	}
}
