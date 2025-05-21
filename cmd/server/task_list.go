package main

import (
	"log"
	"sync"
	"time"

	"internal/pb"
)

type taskState int

const (
	taskState_PENDING taskState = iota
	taskState_DOWNLOADING
	taskState_VALIDATING
	taskState_TRANSFERRING
	taskState_COMPLETED
	taskState_FAILED
)

type taskInfo struct {
	nameOfClient string
	downloadUrl  string
	state        taskState
	checksum     string
	handler      func()
	stream       pb.DDSONService_DownloadServer

	// execution result
	subTasks []subTaskInfo
	err      error
	done     chan bool
}

type subTaskInfo struct {
}

type taskList struct {
	tasks []*taskInfo
	mtx   sync.Mutex
	cond  *sync.Cond
}

func newTaskList() *taskList {
	return &taskList{
		tasks: make([]*taskInfo, 0),
		cond:  sync.NewCond(&sync.Mutex{}),
	}
}

func (t *taskList) addTask(task *taskInfo) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	t.tasks = append(t.tasks, task)
	t.cond.Broadcast() // Notify any waiting goroutines
}

func (t *taskList) size() int {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return len(t.tasks)
}

func (t *taskList) notify() {
	// TODO: notify all pending tasks current state
	log.Printf("TODO: notify all pending tasks current state")
}

func (t *taskList) run() error {
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			t.notify()
		}
	}()

	for {
		t.mtx.Lock()
		for len(t.tasks) == 0 {
			t.cond.Wait() // Wait for tasks to be added
		}

		// get the task on top
		task := t.tasks[0]
		t.tasks = t.tasks[1:] // Remove the task from the list
		t.mtx.Unlock()

		executeTask(task)
	}
}
