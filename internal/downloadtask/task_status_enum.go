package downloadtask

type TaskStatusEnum int

const (
	TaskStatusNew TaskStatusEnum = iota
	TaskStatusPending
	TaskStatusDownloading
	TaskStatusValidating
	TaskStatusTransferring
	TaskStatusCompleted
	TaskStatusFailed
)
