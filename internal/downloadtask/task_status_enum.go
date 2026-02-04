package downloadtask

type TaskStatusEnum int

const (
	TaskStatusNew TaskStatusEnum = iota
	TaskStatusPending
	TaskStatusPreparing
	TaskStatusDownloading
	TaskStatusPostProcessing
	TaskStatusValidating
	TaskStatusDownloadCompleted
	TaskStatusTransferring
	TaskStatusCompleted
	TaskStatusFailed
)
