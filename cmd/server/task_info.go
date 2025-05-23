package main

import (
	"sync"

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
	idOfClient  int //  which client the task is from
	id          int // task ID
	downloadUrl string
	checksum    string
	stream      pb.DDSONService_DownloadServer

	mtx               *sync.Mutex // Mutex to protect access to the task states
	cond              *sync.Cond
	state             taskState
	pendingSubTasks   []*subTaskInfo
	completedSubTasks []*subTaskInfo
	runningSubTasks   map[int32]*subTaskInfo

	err              error
	completeFilePath string
	done             chan bool
}

func newTaskInfo(downloadUrl string, checksum string, stream pb.DDSONService_DownloadServer, taskId int, idOfClient int) *taskInfo {
	mtx := &sync.Mutex{}
	return &taskInfo{
		downloadUrl: downloadUrl,
		checksum:    checksum,
		stream:      stream,
		id:          taskId,
		idOfClient:  idOfClient,

		state:             taskState_PENDING,
		mtx:               mtx,
		cond:              sync.NewCond(mtx),
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
