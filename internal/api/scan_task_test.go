// internal/api/scan_task_test.go
package api

import (
	"context"
	"testing"
)

func TestScanTaskLifecycle(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	task := NewScanTask("batch-1", []string{"a", "b", "c"}, cancel)

	if task.BatchID != "batch-1" {
		t.Errorf("BatchID: got %q, want %q", task.BatchID, "batch-1")
	}
	if task.TotalAgents != 3 {
		t.Errorf("TotalAgents: got %d, want 3", task.TotalAgents)
	}
	if task.Status != "running" {
		t.Errorf("初始 Status 应为 running: got %q", task.Status)
	}

	// Update 后 snapshot
	task.Update(1, "a", nil)
	snap := task.Snapshot()
	if snap.Completed != 1 {
		t.Errorf("Completed: got %d, want 1", snap.Completed)
	}
	if snap.CurrentAgent != "a" {
		t.Errorf("CurrentAgent: got %q, want %q", snap.CurrentAgent, "a")
	}
	if snap.Status != "running" {
		t.Errorf("Status 仍应为 running: got %q", snap.Status)
	}

	// MarkCompleted
	task.MarkCompleted(nil)
	snap2 := task.Snapshot()
	if snap2.Status != "completed" {
		t.Errorf("Status 应为 completed: got %q", snap2.Status)
	}
	if snap2.Completed != 3 {
		t.Errorf("Completed 应为 TotalAgents=3: got %d", snap2.Completed)
	}
}

func TestScanTaskCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	task := NewScanTask("batch-2", []string{"a"}, cancel)

	task.MarkCancelled()
	snap := task.Snapshot()
	if snap.Status != "cancelled" {
		t.Errorf("Status 应为 cancelled: got %q", snap.Status)
	}

	// ctx 应被取消
	select {
	case <-ctx.Done():
		// ok
	default:
		t.Error("cancel 应取消 context")
	}
}

func TestScanTaskSnapshotConcurrency(t *testing.T) {
	// 并发 Update + Snapshot 不 panic(go test -race 检测)
	_, cancel := context.WithCancel(context.Background())
	task := NewScanTask("batch-3", []string{"a", "b"}, cancel)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			task.Update(i%2+1, "agent", nil)
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 100; i++ {
			task.Snapshot()
		}
		done <- struct{}{}
	}()
	<-done
	<-done
}
