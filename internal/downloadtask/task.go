package downloadtask

import (
	"os"
	"sync"

	"github.com/imafish/ddson/internal/agents"
	"github.com/imafish/ddson/internal/pb"
)

// TODO: task could be an interface that represent runnable objects, and have a method Run()
// At present, it is highly coupled with current server implementation.

type Task struct {
	info              TaskInfo
	taskStatusChannel chan TaskStatus

	mtx        *sync.Mutex
	taskStatus *TaskStatus
	subtasks   []*SubTask

	// internal use, to manage task execution
	abortFlag       bool
	tmpFolder       string
	subtaskDoneChan chan int

	// TODO: to be removed
	stream       pb.DDSONService_DownloadServer // TODO task info should not have grpc stream
	errorHandler *ErrorHandler                  // TODO: this field should be in 'task manager class' in the future.
}

func NewTask(downloadUrl string, checksum string, size uint64, stream pb.DDSONService_DownloadServer, taskId int, idOfClient int, agentManager *agents.AgentManager, tmpFolder string) *Task {
	mtx := &sync.Mutex{}
	return &Task{
		info: TaskInfo{
			DownloadUrl: downloadUrl,
			Checksum:    checksum,
			ID:          taskId,
			Size:        size,
		},
		taskStatusChannel: make(chan TaskStatus, 10),

		mtx: mtx,
		taskStatus: &TaskStatus{
			Status: TaskStatusNew,
		},
		subtasks: make([]*SubTask, 0),

		abortFlag:       false,
		tmpFolder:       tmpFolder,
		subtaskDoneChan: make(chan int),

		stream:       stream,
		errorHandler: NewErrorHandlerWithDefaults(agentManager),
	}
}

func (t *Task) GetTaskInfo() TaskInfo {
	return t.info
}

func (t *Task) GetTaskStatusChannel() <-chan TaskStatus {
	return t.taskStatusChannel
}

func (t *Task) GetTaskStatus() TaskStatus {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return *t.taskStatus
}

func (t *Task) fail(err *DownloadError) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	t.taskStatus.Status = TaskStatusFailed
	t.taskStatus.Err = err
	t.abortFlag = true
	t.sendStatusLockedNonBlocking()
}

func (t *Task) sendStatusLockedNonBlocking() {
	select {
	case t.taskStatusChannel <- *t.taskStatus:
	default:
	}
}

func (t *Task) close() {
	close(t.taskStatusChannel)
	os.Remove(t.taskStatus.DownloadedFilePath)
}
