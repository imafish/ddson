package downloadtask

import (
	"errors"
	"sync"
	"testing"
)

func newTestTask(id int) *Task {
	return &Task{
		info:              taskInfo{ID: id, DownloadUrl: "http://example.com", Size: 100},
		taskStatusChannel: make(chan TaskStatus, 1),
		mtx:               &sync.Mutex{},
		taskStatus:        &TaskStatus{Status: TaskStatusNew},
		subtasks:          make([]*subTask, 0),
	}
}

func TestTaskManagerAddAndGetTask(t *testing.T) {
	manager, ok := NewTaskManagerImpl().(*taskManagerImpl)
	if !ok {
		t.Fatalf("expected *taskManagerImpl")
	}

	task1 := newTestTask(0)
	task2 := newTestTask(0)

	id1 := manager.AddTask(task1)
	id2 := manager.AddTask(task2)

	if id1 != 0 || id2 != 1 {
		t.Fatalf("unexpected ids: %d, %d", id1, id2)
	}

	pending := manager.GetPendingTasks()
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending tasks, got %d", len(pending))
	}

	got, err := manager.GetTaskByID(id1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.taskStatus.PositionInQueue != 0 {
		t.Fatalf("expected PositionInQueue=0, got %d", got.taskStatus.PositionInQueue)
	}
}

func TestTaskManagerGetTaskNotFound(t *testing.T) {
	manager, ok := NewTaskManagerImpl().(*taskManagerImpl)
	if !ok {
		t.Fatalf("expected *taskManagerImpl")
	}

	_, err := manager.GetTaskByID(123)
	if err == nil {
		t.Fatalf("expected error")
	}

	var downloadErr *DownloadError
	if !errors.As(err, &downloadErr) {
		t.Fatalf("expected DownloadError, got %T", err)
	}
	if downloadErr.Code != ErrCodeDownloadTaskNotFound {
		t.Fatalf("unexpected error code: %v", downloadErr.Code)
	}
}
