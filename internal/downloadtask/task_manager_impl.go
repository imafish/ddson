package downloadtask

import (
	"context"
	"fmt"
	"sync"
)

type taskManagerImpl struct {
	pendingTasks   []*Task
	runningTasks   map[int]*Task // map of taskID to running tasks
	completedTasks map[int]*Task // map of taskID to completed tasks

	nextTaskID int
	mtx        *sync.Mutex
	cond       *sync.Cond
	stopCh     chan struct{}
	stopped    bool
}

func NewTaskManagerImpl() TaskManager {
	mtx := &sync.Mutex{}
	return &taskManagerImpl{
		pendingTasks:   make([]*Task, 0),
		runningTasks:   make(map[int]*Task),
		completedTasks: make(map[int]*Task),
		mtx:            mtx,
		cond:           sync.NewCond(mtx),
		stopCh:         make(chan struct{}),
	}
}

func (t *taskManagerImpl) GetPendingTasks() []Task {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	tasks := make([]Task, len(t.pendingTasks))
	for i, task := range t.pendingTasks {
		tasks[i] = *task
	}
	return tasks
}

func (t *taskManagerImpl) GetRunningTasks() []Task {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	tasks := make([]Task, 0, len(t.runningTasks))
	for _, task := range t.runningTasks {
		tasks = append(tasks, *task)
	}
	return tasks
}

func (t *taskManagerImpl) GetCompletedTasks() []Task {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	tasks := make([]Task, 0, len(t.completedTasks))
	for _, task := range t.completedTasks {
		tasks = append(tasks, *task)
	}
	return tasks
}

func (t *taskManagerImpl) GetTaskByID(taskID int) (Task, error) {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	if task, ok := t.runningTasks[taskID]; ok {
		return *task, nil
	}
	if task, ok := t.completedTasks[taskID]; ok {
		return *task, nil
	}
	_, task := t.findTaskInPendingTasksByIDLocked(taskID)
	if task != nil {
		return *task, nil
	}

	return Task{}, NewDownloadErrorWithMessage(ErrCodeDownloadTaskNotFound, fmt.Sprintf("Task with ID %d not found", taskID))
}

func (t *taskManagerImpl) AddTask(task *Task) int {
	t.mtx.Lock()

	newId := t.nextTaskID
	task.info.ID = newId
	t.nextTaskID++

	t.pendingTasks = append(t.pendingTasks, task)
	t.mtx.Unlock()
	t.cond.Broadcast() // Notify any waiting goroutines

	return newId
}

// TODO: add a Stop() method
func (t *taskManagerImpl) Run(ctx context.Context, fn func(task *Task) error) {
	if ctx == nil {
		ctx = context.Background()
	}

	go func() {
		<-ctx.Done()
		t.Stop()
	}()

	for {
		t.mtx.Lock()
		for len(t.pendingTasks) == 0 && !t.stopped {
			logger.Info("task list empty, waiting...")
			t.cond.Wait() // Wait for tasks to be added
		}
		if t.stopped {
			t.mtx.Unlock()
			return
		}

		// get the task on top
		task := t.pendingTasks[0]
		id := task.info.ID
		t.pendingTasks = t.pendingTasks[1:] // Remove the task from the list
		t.runningTasks[id] = task
		t.mtx.Unlock()

		logger.Info("Got a task to run", "taskID", id, "url", task.info.DownloadUrl, "checksum", task.info.Checksum)

		err := fn(task)

		t.mtx.Lock()
		// remove from running tasks
		delete(t.runningTasks, id)
		if err == nil {
			// move to completed tasks
			t.completedTasks[id] = task
		} else {
			// TODO: maybe retry later
			logger.Error("Task failed", "taskID", id, "error", err)
		}
		t.mtx.Unlock()
	}
}

func (t *taskManagerImpl) Stop() {
	t.mtx.Lock()
	if t.stopped {
		t.mtx.Unlock()
		return
	}
	t.stopped = true
	close(t.stopCh)
	t.mtx.Unlock()
	t.cond.Broadcast()
}

func (t *taskManagerImpl) findTaskInPendingTasksByIDLocked(taskID int) (int, *Task) {
	for index, task := range t.pendingTasks {
		if task.info.ID == taskID {
			task.taskStatus.PositionInQueue = index
			return index, task
		}
	}
	return -1, nil
}
