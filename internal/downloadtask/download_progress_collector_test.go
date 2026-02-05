package downloadtask

import (
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	return c.now
}

func newTestTaskForCollector() *Task {
	return &Task{
		info:              taskInfo{ID: 1, DownloadUrl: "http://example.com", Size: 100},
		taskStatusChannel: make(chan TaskStatus, 10),
		mtx:               &sync.Mutex{},
		taskStatus:        &TaskStatus{Status: TaskStatusNew},
		subtasks:          make([]*subTask, 3),
	}
}

func TestDownloadProgressCollectorLifecycle(t *testing.T) {
	task := newTestTaskForCollector()
	clock := &fakeClock{now: time.Now()}
	collector := newDownloadStatusCollector(task, clock)

	collector.lastSpeedUpdate = clock.Now().Add(-time.Second)
	collector.onSubtaskStart(1, 10)

	status := task.GetTaskStatus()
	if status.DownloadingParts != 1 {
		t.Fatalf("expected DownloadingParts=1, got %d", status.DownloadingParts)
	}

	clock.now = clock.now.Add(600 * time.Millisecond)
	collector.onSubtaskProgress(1, 50)
	status = task.GetTaskStatus()
	if status.TotalDownloadedBytes != 50 {
		t.Fatalf("expected TotalDownloadedBytes=50, got %d", status.TotalDownloadedBytes)
	}

	collector.onSubtaskCompleted(1, "/tmp/file")
	status = task.GetTaskStatus()
	if status.DownloadedParts != 1 || status.DownloadingParts != 0 {
		t.Fatalf("expected DownloadedParts=1 and DownloadingParts=0, got %d and %d", status.DownloadedParts, status.DownloadingParts)
	}
}

func TestDownloadProgressCollectorErrorResetsBytes(t *testing.T) {
	task := newTestTaskForCollector()
	clock := &fakeClock{now: time.Now()}
	collector := newDownloadStatusCollector(task, clock)

	collector.lastSpeedUpdate = clock.Now().Add(-time.Second)
	collector.onSubtaskStart(2, 20)
	clock.now = clock.now.Add(600 * time.Millisecond)
	collector.onSubtaskProgress(2, 80)

	collector.onSubtaskError(2, NewDownloadErrorWithMessage(ErrCodeDownloadInterrupted, "test"))
	status := task.GetTaskStatus()
	if status.DownloadingParts != 0 {
		t.Fatalf("expected DownloadingParts=0, got %d", status.DownloadingParts)
	}
	if status.TotalDownloadedBytes != 0 {
		t.Fatalf("expected TotalDownloadedBytes=0, got %d", status.TotalDownloadedBytes)
	}
}
