// internal/api/scan_task.go
package api

import (
	"context"
	"sync"
)

// ScanTask 表示一次异步扫描任务的内存态追踪数据。
// 任务在 POST /api/scan 时创建,存储在 Server.scanTasks sync.Map 中。
// goroutine 每完成一个 agent 调 Update;全部完成调 MarkCompleted;
// 用户取消调 MarkCancelled。完成/取消后 5 分钟自动从 sync.Map 删除。
type ScanTask struct {
	BatchID      string
	AgentIDs     []string
	TotalAgents  int
	Completed    int
	CurrentAgent string
	Status       string // "running" | "completed" | "cancelled"
	Results      []AgentScanResult
	cancel       context.CancelFunc
	mu           sync.Mutex
}

// ScanTaskSnapshot 是 ScanTask 的只读快照,用于 GET /api/scan/progress 响应。
type ScanTaskSnapshot struct {
	BatchID      string            `json:"batch_id"`
	Status       string            `json:"status"`
	TotalAgents  int               `json:"total_agents"`
	Completed    int               `json:"completed_agents"`
	CurrentAgent string            `json:"current_agent"`
	Results      []AgentScanResult `json:"results,omitempty"`
}

// NewScanTask 创建新任务,初始 Status="running"。
func NewScanTask(batchID string, agentIDs []string, cancel context.CancelFunc) *ScanTask {
	return &ScanTask{
		BatchID:     batchID,
		AgentIDs:    agentIDs,
		TotalAgents: len(agentIDs),
		Status:      "running",
		cancel:      cancel,
	}
}

// Update 更新进度(完成数、当前 agent、已有结果)。goroutine-safe。
func (t *ScanTask) Update(completed int, currentAgent string, results []AgentScanResult) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Completed = completed
	t.CurrentAgent = currentAgent
	t.Results = results
}

// MarkCompleted 标记完成,设置 Status="completed"。goroutine-safe。
func (t *ScanTask) MarkCompleted(results []AgentScanResult) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = "completed"
	t.Completed = t.TotalAgents
	t.Results = results
}

// MarkCancelled 标记取消,调用 context.CancelFunc 中止扫描。goroutine-safe。
func (t *ScanTask) MarkCancelled() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = "cancelled"
	if t.cancel != nil {
		t.cancel()
	}
}

// Snapshot 返回只读快照(不含 results)。
func (t *ScanTask) Snapshot() ScanTaskSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return ScanTaskSnapshot{
		BatchID:      t.BatchID,
		Status:       t.Status,
		TotalAgents:  t.TotalAgents,
		Completed:    t.Completed,
		CurrentAgent: t.CurrentAgent,
	}
}

// SnapshotWithResults 返回只读快照(含 results,仅在完成时调用)。
func (t *ScanTask) SnapshotWithResults() ScanTaskSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := ScanTaskSnapshot{
		BatchID:      t.BatchID,
		Status:       t.Status,
		TotalAgents:  t.TotalAgents,
		Completed:    t.Completed,
		CurrentAgent: t.CurrentAgent,
		Results:      t.Results,
	}
	return s
}
