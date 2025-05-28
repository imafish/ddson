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

	mtx      *sync.Mutex // Mutex to protect access to the task states
	state    taskState
	subtasks []*subTaskInfo

	err  error
	done chan bool
}

func newTaskInfo(downloadUrl string, checksum string, stream pb.DDSONService_DownloadServer, taskId int, idOfClient int) *taskInfo {
	mtx := &sync.Mutex{}
	return &taskInfo{
		downloadUrl: downloadUrl,
		checksum:    checksum,
		stream:      stream,
		id:          taskId,
		idOfClient:  idOfClient,

		state:    taskState_PENDING,
		mtx:      mtx,
		subtasks: make([]*subTaskInfo, 0),
		err:      nil,
		done:     make(chan bool),
	}
}

// setError sets the error for the task and updates its state to FAILED
func (t *taskInfo) setError(err error) {
	t.err = err
	t.state = taskState_FAILED
}

// setState sets the state of the task and notifies any waiting goroutines
func (t *taskInfo) markDone() {
	close(t.done)
}
