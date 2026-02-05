package downloadtask

import (
	"sync"
	"time"

	"github.com/imafish/ddson/internal/agents"
	"github.com/imafish/ddson/internal/common"
	"github.com/imafish/ddson/internal/pb"
)

// TODO: task could be an interface that represent runnable objects, and have a method Run()
// At present, it is highly coupled with current server implementation.

type Task struct {
	info              taskInfo
	taskStatusChannel chan TaskStatus

	mtx        *sync.Mutex
	taskStatus *TaskStatus
	subtasks   []*subTask

	// internal use, to manage task execution
	abortFlag               bool     // to signal subtasks to abort
	tmpFolder               string   // temporary folder to store subtask files
	subtaskDoneChan         chan int // subtasks use this channel to report completion
	errorHandler            *errorHandler
	agentManager            agents.AgentManager
	downloadStatusCollector *downloadProgressCollector
	clock                   common.Clock
	subtaskExecutor         SubtaskExecutor
	fs                      common.FileSystem
}

func NewTask(downloadUrl string, checksum string, size uint64, stream pb.DDSONService_DownloadServer, taskId int, idOfClient int, agentManager agents.AgentManager) *Task {
	mtx := &sync.Mutex{}
	task := &Task{
		info: taskInfo{
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
		subtasks: make([]*subTask, 0),

		abortFlag:       false,
		tmpFolder:       "",
		subtaskDoneChan: make(chan int),
		errorHandler:    newErrorHandlerWithDefaults(agentManager),
		agentManager:    agentManager,
		clock:           common.RealClock{},
		subtaskExecutor: defaultSubtaskExecutor{},
		fs:              common.OSFileSystem{},
	}
	task.downloadStatusCollector = newDownloadStatusCollector(task, task.clock)
	return task
}

func (t *Task) Run() error {
	return runDownloadTask(t)
}

func (t *Task) GetTaskStatusChannel() <-chan TaskStatus {
	return t.taskStatusChannel
}

func (t *Task) GetTaskStatus() TaskStatus {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return *t.taskStatus
}

func (t *Task) updateAndSendTaskStatus(args ...any) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	logger.Debug("UpdateAndSendTaskStatus", args...)

	argLength := len(args)
	if argLength < 2 {
		panic("UpdateAndSendTaskStatus requires at least 2 arguments")
	}
	if argLength%2 != 0 {
		panic("UpdateAndSendTaskStatus requires even number of arguments")
	}

	for i := 0; i < argLength; i += 2 {
		field := args[i].(string)
		value := args[i+1]

		switch field {
		case "Status":
			t.taskStatus.Status = value.(TaskStatusEnum)
		case "Err":
			t.taskStatus.Err = value.(*DownloadError)
		case "DownloadedFilePath":
			t.taskStatus.DownloadedFilePath = value.(string)
		case "PositionInQueue":
			t.taskStatus.PositionInQueue = value.(int)
		case "TotalParts":
			t.taskStatus.TotalParts = value.(int)
		case "DownloadedParts":
			t.taskStatus.DownloadedParts = value.(int)
		case "DownloadingParts":
			t.taskStatus.DownloadingParts = value.(int)
		case "DownloadSpeed":
			t.taskStatus.DownloadSpeed = value.(int)
		case "TotalDownloadedBytes":
			t.taskStatus.TotalDownloadedBytes = value.(int64)
		case "EstimatedRemainingTime":
			t.taskStatus.EstimatedRemainingTime = value.(time.Duration)
		default:
			logger.Warn("Unknown task status field", "field", field)
		}
	}
	t.sendStatusLockedNonBlocking()
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

func (t *Task) Cleanup() {
	if t.tmpFolder != "" {
		t.fs.RemoveAll(t.tmpFolder)
		t.tmpFolder = ""
	}
	if t.taskStatus.DownloadedFilePath != "" {
		t.fs.Remove(t.taskStatus.DownloadedFilePath)
		t.taskStatus.DownloadedFilePath = ""
	}
	close(t.taskStatusChannel)
}
