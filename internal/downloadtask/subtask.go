package downloadtask

type subTask struct {
	downloadUrl  string
	id           int
	offset       uint64
	downloadSize uint64
	targetFile   string
	err          *DownloadError
}

func newDownloadSubTask(downloadUrl string, id int, offset uint64, downloadSize uint64, targetFile string) *subTask {
	return &subTask{
		downloadUrl:  downloadUrl,
		id:           id,
		offset:       offset,
		downloadSize: downloadSize,
		targetFile:   targetFile,
		err:          nil,
	}
}
