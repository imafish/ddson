package main

import "time"

// TODO: downloadProgress will return incorrect result under the following conditions:
// 1. if a client fails. its downloaded bytes will not be removed.
// Should consider using subtaskID instead of clientID to track progress, and handle situations where a client fails.
type downloadProgress struct {
	startTime                  time.Time
	clientTotalDownloadedBytes map[int]int
}

func newDownloadProgress() *downloadProgress {
	return &downloadProgress{
		startTime:                  time.Now(),
		clientTotalDownloadedBytes: make(map[int]int),
	}
}

// TODO: no error handling: if any agent fails, the task fails for now.
// later we should be able to retry with a different agent
// downloadProgress should be able to handle this error
// TODO: this speed is not accurate
// TODO: consider adding a method to get periodic speed instead of speed since start
func (dp *downloadProgress) getTotalSpeed() int {
	now := time.Now()
	elapsed := now.Sub(dp.startTime).Seconds()
	if elapsed > 0 {
		totalDownloaded := dp.getTotalDownloadedBytes()
		return int(float64(totalDownloaded) / elapsed)
	}
	return 0
}

func (dp *downloadProgress) getTotalDownloadedBytes() int {
	totalDownloaded := 0
	for _, downloaded := range dp.clientTotalDownloadedBytes {
		totalDownloaded += downloaded
	}
	return totalDownloaded
}

func (dp *downloadProgress) updateProgress(clientId int, bytesDownloaded int) {
	dp.clientTotalDownloadedBytes[clientId] += bytesDownloaded
}
