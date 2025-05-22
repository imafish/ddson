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
	checksum     string
	stream       pb.DDSONService_DownloadServer

	mtx               sync.Mutex // Mutex to protect access to the task states
	cond              *sync.Cond
	state             taskState
	pendingSubTasks   []*subTaskInfo
	completedSubTasks []*subTaskInfo
	runningSubTasks   map[int32]*subTaskInfo

	err              error
	completeFilePath string
	done             chan bool
}

func newTaskInfo(nameOfClient string, downloadUrl string, checksum string, stream pb.DDSONService_DownloadServer) *taskInfo {
	return &taskInfo{
		nameOfClient:      nameOfClient,
		downloadUrl:       downloadUrl,
		checksum:          checksum,
		state:             taskState_PENDING,
		stream:            stream,
		mtx:               sync.Mutex{},
		cond:              sync.NewCond(&sync.Mutex{}),
		pendingSubTasks:   make([]*subTaskInfo, 0),
		runningSubTasks:   make(map[int32]*subTaskInfo),
		completedSubTasks: make([]*subTaskInfo, 0),
		err:               nil,
		completeFilePath:  "",
		done:              make(chan bool),
	}
}

// setError sets the error for the task and updates its state to FAILED, then notifies any waiting goroutines
func (t *taskInfo) setError(err error) {
	t.err = err
	t.state = taskState_FAILED
	t.cond.Broadcast() // Notify any waiting goroutines
}

// setState sets the state of the task and notifies any waiting goroutines
func (t *taskInfo) markDone() {
	t.cond.Broadcast()
	close(t.done)
}

type subTaskInfo struct {
	downloadUrl  string
	id           int32
	offset       int64
	downloadSize int64
	state        taskState
	assignedTo   int32
	targetFile   string
}

func newSubTaskInfo(downloadUrl string, id int32, offset int64, downloadSize int64, targetFile string) *subTaskInfo {
	return &subTaskInfo{
		downloadUrl:  downloadUrl,
		id:           id,
		offset:       offset,
		downloadSize: downloadSize,
		state:        taskState_PENDING,
		assignedTo:   -1,
		targetFile:   targetFile,
	}
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

func (t *taskList) run(server *server) error {
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

		executeTask(task, server)
	}
}
