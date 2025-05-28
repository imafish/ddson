package main

import (
	"internal/pb"
	"log/slog"
	"sync"
)

type taskList struct {
	tasks  []*taskInfo
	freeId int
	mtx    *sync.Mutex
	cond   *sync.Cond
}

func newTaskList() *taskList {
	mtx := &sync.Mutex{}
	return &taskList{
		tasks: make([]*taskInfo, 0),
		mtx:   mtx,
		cond:  sync.NewCond(mtx),
	}
}

func (t *taskList) addTask(downloadUrl string, checksum string, stream pb.DDSONService_DownloadServer, idOfClient int) *taskInfo {
	t.mtx.Lock()
	newId := t.freeId
	t.freeId++

	task := newTaskInfo(downloadUrl, checksum, stream, newId, idOfClient)
	t.tasks = append(t.tasks, task)
	t.mtx.Unlock()
	t.cond.Broadcast() // Notify any waiting goroutines

	return task
}

func (t *taskList) size() int {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return len(t.tasks)
}

func (t *taskList) run(server *server) error {
	for {
		t.mtx.Lock()
		for len(t.tasks) == 0 {
			slog.Info("task list empty, waiting...")
			t.cond.Wait() // Wait for tasks to be added
		}

		// get the task on top
		task := t.tasks[0]
		t.tasks = t.tasks[1:] // Remove the task from the list
		t.mtx.Unlock()

		slog.Info("Got a task to run", "taskID", task.id, "clientID", task.idOfClient, "url", task.downloadUrl, "checksum", task.checksum)

		executeTask(task, server)
	}
}
