// Package scheduler 实现进程内定时扫描(time.Ticker + goroutine)。
// 服务运行期间按配置间隔自动扫描;inFlight 互斥防重叠;Stop/Reconfigure 可安全启停。
package scheduler

import (
	"context"
	"sync"
	"time"
)

type Scheduler struct {
	interval time.Duration
	run      func(context.Context) error

	mu       sync.Mutex // 保护 inFlight / running / lastRun / nextRun
	ticker   *time.Ticker
	cancel   context.CancelFunc
	inFlight bool
	running  bool
	lastRun  time.Time
	nextRun  time.Time

	// wg 跟踪进行中的 tick,使 Stop 能等其结束。
	// 计划稿原实现未等待 in-flight tick,导致 "Stop 后不再触发" 不成立:
	// loop 收到 tick 后、tick 真正调 run 前,Stop 可能已返回;run 随后完成并
	// 递增计数,触发 TestSchedulerTicksAndStops 的 "Stop 后不应再触发" 断言。
	// 配合 tick 内的 !s.running 检查(防 Stop 后新 tick 启动)共同保证语义。
	wg sync.WaitGroup
}

// SchedulerStatus 是 GET /api/scheduler 的响应。
type SchedulerStatus struct {
	Enabled  bool          `json:"enabled"`
	Interval time.Duration `json:"interval"`
	LastRun  time.Time     `json:"last_run"`
	NextRun  time.Time     `json:"next_run"`
}

// New 构造 Scheduler(未启动)。interval <= 0 表示关闭。
func New(interval time.Duration, run func(context.Context) error) *Scheduler {
	return &Scheduler{interval: interval, run: run}
}

// Start 起 goroutine 按 interval 周期调 run。interval <= 0 时不启动。
// 重复 Start 安全(已运行则 no-op)。
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running || s.interval <= 0 {
		return
	}
	s.ticker = time.NewTicker(s.interval)
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.running = true
	s.nextRun = time.Now().Add(s.interval)
	go s.loop(ctx)
}

func (s *Scheduler) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.ticker.C:
			s.tick(ctx)
		}
	}
}

// tick 执行一次 run;inFlight 时跳过(non-blocking 防重叠)。
// 额外检查 !s.running:Stop 置 running=false 后,loop 已收到的 tick 不再触发 run。
func (s *Scheduler) tick(ctx context.Context) {
	s.mu.Lock()
	if s.inFlight || !s.running {
		s.mu.Unlock()
		return // 上次未结束或已 Stop,跳过本次 tick
	}
	s.inFlight = true
	s.wg.Add(1)
	s.mu.Unlock()

	_ = s.run(ctx)

	s.mu.Lock()
	s.inFlight = false
	s.lastRun = time.Now()
	s.nextRun = s.lastRun.Add(s.interval)
	s.wg.Done()
	s.mu.Unlock()
}

// Stop 停止 ticker 与 goroutine。可重复调用。
// 会等待进行中的 tick 结束,以保证 "Stop 后不再有 run 完成"。
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.running = false
	s.mu.Unlock()
	// 在锁外等待 in-flight tick:tick 需取 mu 才能调 wg.Done。
	s.wg.Wait()
}

// Status 返回当前调度状态(线程安全)。
func (s *Scheduler) Status() SchedulerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SchedulerStatus{
		Enabled:  s.running,
		Interval: s.interval,
		LastRun:  s.lastRun,
		NextRun:  s.nextRun,
	}
}

// Reconfigure 改间隔 / 启停:先 Stop,再按 enabled+interval 决定是否 Start。
// 供 PUT /api/scheduler 调用。
func (s *Scheduler) Reconfigure(enabled bool, interval time.Duration) {
	s.Stop()
	s.mu.Lock()
	s.interval = interval
	s.mu.Unlock()
	if enabled && interval > 0 {
		s.Start()
	}
}
