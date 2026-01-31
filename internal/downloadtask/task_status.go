package downloadtask

// TODO: TaskStatus could also be an interface to represent more common task status methods.
// At present, it is highly coupled with current server implementation.

type TaskStatus struct {
	Status             TaskStatusEnum
	Err                *DownloadError
	DownloadedFilePath string

	PositionInQueue int // pending

	// downloading
	TotalParts       int
	DownloadedParts  int
	DownloadingParts int
	DownloadSpeed    int // in bytes per second
}
