package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"log/slog"

	"internal/common"
	"internal/httputil"
	"internal/pb"
)

const CHUNK_SIZE = int64(10 * 1024 * 1024) // 10 MB

func executeTask(task *taskInfo, server *server) {
	defer task.markDone()

	// Check if server supports partial downloads
	supportsPartial, totalSize, err := httputil.CheckPartialDownloadSupport(task.downloadUrl)
	if err != nil {
		slog.Error("Error checking partial download support", "error", err)
		task.setError(err)
		return
	}
	if !supportsPartial {
		slog.Warn("Server does not support partial downloads, downloading the whole file")
		err = fmt.Errorf("server does not support partial downloads")
		task.setError(err)
		return
	}

	// create temporary folder in /tmp
	tmpDir, err := os.MkdirTemp("", "ddson")
	slog.Info("saving temporary files", "dir", tmpDir)
	if err != nil {
		slog.Error("Error creating temporary directory", "error", err)
		task.setError(err)
		return
	}
	defer os.RemoveAll(tmpDir) // Clean up temporary directory

	progressChan := make(chan [2]int, 32)
	defer close(progressChan)

	// start a goroutine to update the download progress
	go progressFunc(progressChan, task)

	// create sub tasks
	task.subtasks = createSubtasks(task.downloadUrl, tmpDir, totalSize, progressChan)
	totalSubTasks := len(task.subtasks)
	slog.Info("Created sub tasks", "count", totalSubTasks)

	err = executeSubTasks(task, server)

	if err != nil {
		slog.Error("Error executing sub tasks", "error", err)
		task.setError(err)
		return
	}
	slog.Info("All sub tasks executed", "count", totalSubTasks)

	completeFile, err := combine(task.subtasks, totalSize)
	if err != nil {
		slog.Error("Error combining files", "error", err)
		task.setError(err)
		return
	}
	slog.Info("Combined file created", "file", completeFile)

	if task.checksum != "" {
		slog.Info("Validating combined file", "file", completeFile, "checksum", task.checksum)
		err = task.stream.Send(&pb.DownloadStatus{
			Status: pb.DownloadStatusType_VALIDATING,
		})
		if err != nil {
			slog.Error("Failed to send validation status", "error", err)
			task.setError(err)
			return
		}
		err = validateFile(completeFile, task.checksum)
		if err != nil {
			slog.Error("Error validating file", "error", err)
			task.setError(err)
			return
		}
	} else {
		slog.Info("No checksum provided, skipping validation")
	}

	defer os.Remove(completeFile) // Clean up combined file after transfer
	err = transferFileData(task.stream, completeFile)
	if err != nil {
		slog.Error("Error transferring file data", "error", err)
		task.setError(err)
		return
	}
	slog.Info("File transfer completed", "file", completeFile)

	// move the downloaded file to a temporary location, and save the path of taskInfo
	tempFile, err := os.CreateTemp("", "downloaded_")
	if err != nil {
		slog.Error("Error creating temporary file for downloaded content", "error", err)
	} else {
		defer tempFile.Close()

		os.Rename(completeFile, tempFile.Name())
		slog.Info("Moved combined file to temporary location", "file", tempFile.Name())
		task.downloadedFile = tempFile.Name()
	}

	task.state = taskState_COMPLETED
}

func combine(subtasks []*subTaskInfo, totalSize int64) (string, error) {
	// Create a new file to write the combined content
	combinedFile, err := os.CreateTemp("", "combined_")
	if err != nil {
		slog.Error("Error creating combined file", "error", err)
		return "", err
	}
	defer combinedFile.Close()
	slog.Info("Combining sub tasks into file", "file", combinedFile.Name())

	// sort the completed sub tasks by offset
	sort.Slice(subtasks, func(i, j int) bool {
		return subtasks[i].offset < subtasks[j].offset
	})

	// print sorted sub tasks
	for _, subTask := range subtasks {
		slog.Debug("Sub task info", "subtaskID", subTask.id, "offset", subTask.offset, "size", subTask.downloadSize, "file", subTask.targetFile)
	}

	currentOffset := int64(0)
	for _, subTask := range subtasks {
		if subTask.offset != currentOffset {
			slog.Error("Error: subtask offset mismatch", "got", subTask.offset, "want", currentOffset)
			return "", fmt.Errorf("subtask offset mismatch: got %d, want %d", subTask.offset, currentOffset)
		}
		currentOffset += subTask.downloadSize
	}

	currentOffset = 0
	for _, subTask := range subtasks {
		slog.Debug("Combining sub task", "subtaskID", subTask.id, "offset", subTask.offset, "size", subTask.downloadSize)
		// Open the sub task file
		file, err := os.Open(subTask.targetFile)
		if err != nil {
			slog.Error("Error opening sub task file", "error", err)
			return "", err
		}
		defer file.Close()
		// Read the content and write it to the combined file
		_, err = io.Copy(combinedFile, file)
		if err != nil {
			slog.Error("Error writing to combined file", "error", err)
			return "", err
		}
		// Update the current offset
		currentOffset += subTask.downloadSize
	}

	if currentOffset != totalSize {
		slog.Error("Error: total size mismatch", "got", currentOffset, "want", totalSize)
		return "", fmt.Errorf("total size mismatch: got %d, want %d", currentOffset, totalSize)
	}

	return combinedFile.Name(), nil
}

func validateFile(file string, checksum string) error {
	// calculate the checksum (sha256) of the file
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		slog.Error("Error calculating checksum", "error", err)
		return err
	}
	sum := hex.EncodeToString(hasher.Sum(nil))
	if sum != checksum {
		slog.Error("Checksum mismatch", "got", sum, "want", checksum)
		return fmt.Errorf("checksum mismatch: got %s, want %s", sum, checksum)
	}
	return nil
}

func progressFunc(progressChan chan [2]int, task *taskInfo) {
	downloadProgress := newDownloadProgress()
	lastUpdate := time.Now()

	for progress := range progressChan {
		clientId, bytesDownloaded := progress[0], progress[1]
		downloadProgress.updateProgress(clientId, bytesDownloaded)

		now := time.Now()
		if now.Sub(lastUpdate) > 2*time.Second {
			lastUpdate = now

			totalSpeed := downloadProgress.getTotalSpeed()
			slog.Debug("Total download speed", "speed", common.PrettyFormatSpeed(totalSpeed))
			err := task.stream.Send(&pb.DownloadStatus{
				Status:               pb.DownloadStatusType_DOWNLOADING,
				Speed:                int32(totalSpeed),
				TotalDownloadedBytes: int64(downloadProgress.getTotalDownloadedBytes()),
			})
			if err != nil {
				slog.Error("Failed to send download status", "error", err)
				task.setError(err)
				return
			}
		}
	}
}

func createSubtasks(downloadUrl string, tmpDir string, totalSize int64, progressChan chan [2]int) []*subTaskInfo {
	subtasks := make([]*subTaskInfo, 0, totalSize/CHUNK_SIZE+1)
	i := 0
	for offset := int64(0); offset < totalSize; offset += CHUNK_SIZE {
		downloadSize := CHUNK_SIZE
		if offset+downloadSize > totalSize {
			downloadSize = totalSize - offset
		}

		targetFile := fmt.Sprintf("%s/%d", tmpDir, offset)
		subTask := newSubTaskInfo(downloadUrl, i, offset, downloadSize, targetFile, progressChan)
		i++
		subtasks = append(subtasks, subTask)
	}

	return subtasks
}

func executeSubTasks(task *taskInfo, server *server) error {
	totalSubTasks := len(task.subtasks)
	finishChan := make(chan int, totalSubTasks)
	finishedSubTasks := 0
	debugFinishedTasks := make([]int, totalSubTasks)

	for _, subTask := range task.subtasks {
		go subTask.execute(server, &task.quitFlag, finishChan)
	}

	var err error
	for subtaskID := range finishChan {
		finishedSubTasks++
		debugFinishedTasks[subtaskID] = 1 // for debugging purposes
		debugFinishedString := getDebugFinishedString(debugFinishedTasks, totalSubTasks)
		slog.Debug("debug", "debugFinishedTasks", debugFinishedString)

		slog.Info("Subtask completed", "subtaskID", subtaskID, "finishedSubTasks", finishedSubTasks)
		// TODO: subtaskID is also the index in the subtasks slice. Consider using a map?
		subTask := task.subtasks[subtaskID]
		if subTask.err != nil {
			if err == nil {
				// only set the first error
				err = subTask.err
			}
			slog.Error("Subtask failed", "subtaskID", subTask.id, "error", subTask.err)
			task.setError(err)
		}

		// always wait for all subtasks to finish, even if one fails
		// this is to prevent subtask to write to progressChan after quitFlag is set
		if finishedSubTasks == totalSubTasks {
			slog.Info("All sub tasks finished", "taskID", task.id)
			close(finishChan)
			break
		}
	}

	return err
}

func transferFileData(stream pb.DDSONService_DownloadServer, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		slog.Error("Error opening file", "error", err)
		return err
	}
	defer file.Close()

	fileStat, err := file.Stat()
	if err != nil {
		slog.Error("Error getting file info", "error", err)
		return err
	}
	fileSize := fileStat.Size()

	slog.Info("Sending file", "path", filePath, "size", fileSize)
	buffer := make([]byte, 1024*1024) // 1 MB buffer
	totalBytesSent := 0
	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			slog.Error("Error reading file", "error", err)
			return err
		}
		slog.Log(context.Background(), slog.LevelDebug-1, "Sending bytes", "count", n, "totalSent", totalBytesSent)
		err = stream.Send(&pb.DownloadStatus{
			Status: pb.DownloadStatusType_TRANSFERRING,
			Data:   buffer[:n],
		})
		if err != nil {
			slog.Error("Error sending file data", "error", err)
			return err
		}
		totalBytesSent += n
	}
	return nil
}

func getDebugFinishedString(debugFinishedTasks []int, totalSubTasks int) string {
	var debugFinishedTaskBuffer = make([]byte, 0, totalSubTasks*3)
	debugFinishedTaskBuffer = append(debugFinishedTaskBuffer, '[')
	for i, finished := range debugFinishedTasks {
		if finished == 1 {
			debugFinishedTaskBuffer = append(debugFinishedTaskBuffer, fmt.Sprintf("%d ", i)...)
		}
	}
	if len(debugFinishedTaskBuffer) > 1 && debugFinishedTaskBuffer[len(debugFinishedTaskBuffer)-1] == ' ' {
		debugFinishedTaskBuffer = debugFinishedTaskBuffer[:len(debugFinishedTaskBuffer)-1]
	}
	debugFinishedTaskBuffer = append(debugFinishedTaskBuffer, ']')
	return string(debugFinishedTaskBuffer)
}
