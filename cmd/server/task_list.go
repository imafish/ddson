package main

import (
	"internal/pb"
	"log"
	"sync"
	"time"
)

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
			log.Printf("task list empty, waiting...")
			t.cond.Wait() // Wait for tasks to be added
		}

		// get the task on top
		task := t.tasks[0]
		t.tasks = t.tasks[1:] // Remove the task from the list
		t.mtx.Unlock()

		log.Printf("Got a task to run. id: %d, from client: #%d, url: %s, checksum: %s", task.id, task.idOfClient, task.downloadUrl, task.checksum)

		executeTask(task, server)
	}
}
