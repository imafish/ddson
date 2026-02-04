package downloadtask

type taskInfo struct {
	ID          int
	DownloadUrl string
	Checksum    string
	Size        uint64
}
