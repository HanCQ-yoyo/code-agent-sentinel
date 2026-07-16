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

	mu       sync.Mutex // 保护 inFlight / running / lastRun / nextRun / loopDone
	ticker   *time.Ticker
	cancel   context.CancelFunc
	inFlight bool
	running  bool
	lastRun  time.Time
	nextRun  time.Time

	// loopDone 在 Start 时创建,作为局部变量传给 loop goroutine;loop 退出时 close。
	// Stop 等 loopDone 关闭,即等 loop goroutine 完全退出(而非仅等 in-flight tick)。
	// 这样 Reconfigure 紧接 Start 写 s.ticker 时旧 loop 已死,消除 s.ticker 上的数据竞争,
	// 并杜绝旧 loop 用已取消 ctx 触发陈旧 run。
	// 之前的 wg sync.WaitGroup 方案只等 in-flight tick 不等 loop 退出:cancel() 后 loop 仍
	// 可能从 ticker.C 收到缓冲 tick(time.Ticker.Stop 不排空大小为 1 的缓冲)并重新进入
	// select 读 s.ticker.C,与新 Start 写 s.ticker 无 happens-before → 数据竞争。
	loopDone chan struct{}
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
	// loopDone 作为局部变量传给 loop,而非让 loop 读 s.loopDone 字段:
	// 避免 Reconfigure 替换字段后旧 loop defer close 的是新 chan。
	done := make(chan struct{})
	s.loopDone = done
	go s.loop(ctx, done)
}

// loop 是调度主循环。done 在退出时 close,供 Stop 等待 loop 完全退出。
// done 显式传参(局部 chan),不是 s.loopDone 字段,确保 defer close 的是 loop 启动时
// 绑定的那个 chan,即便 Reconfigure 随后替换了 s.loopDone 字段也不受影响。
func (s *Scheduler) loop(ctx context.Context, done chan struct{}) {
	defer close(done)
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
// 额外检查 !s.running:Stop 置 running=false 后,loop 已收到的缓冲 tick 不再触发 run。
// (即便 Stop 已等 loop 退出,保留此检查作为纵深防御:loop 退出前收到的最后一个 tick 若
// 在 Stop 置 running=false 之后被调度到这里,仍不会启动新 run。)
func (s *Scheduler) tick(ctx context.Context) {
	s.mu.Lock()
	if s.inFlight || !s.running {
		s.mu.Unlock()
		return // 上次未结束或已 Stop,跳过本次 tick
	}
	s.inFlight = true
	s.mu.Unlock()

	_ = s.run(ctx)

	s.mu.Lock()
	s.inFlight = false
	s.lastRun = time.Now()
	s.nextRun = s.lastRun.Add(s.interval)
	s.mu.Unlock()
}

// Stop 停止 ticker 与 goroutine。可重复调用。
// 在锁内 cancel + 停 ticker + 置 running=false 并捕获 loopDone chan,锁外等 loop
// 完全退出。等 loop 退出即同时覆盖"等待进行中 tick"(loop 在 tick 返回前无法退出 →
// loopDone 不会关闭),并消除 s.ticker 上的竞争(旧 loop 已死,新 Start 写 s.ticker 时无并发读)。
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
	done := s.loopDone // 捕获到局部变量,锁外等待;close 不需要 mu
	s.mu.Unlock()
	// 等待 loop goroutine 完全退出:defer close(done) 在 loop 返回时执行。
	// done 必非 nil(仅在 running=true 且 Start 过时进入此分支,Start 必创建 loopDone)。
	<-done
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
