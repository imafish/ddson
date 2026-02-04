package downloadtask

type TaskInfo struct {
	ID          int
	DownloadUrl string
	Checksum    string
	Size        uint64
}
