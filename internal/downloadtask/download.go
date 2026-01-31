package downloadtask

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"

	"github.com/imafish/ddson/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const CHUNK_SIZE = uint64(10 * 1024 * 1024) // 10 MB

// run the download task.
// execution status is reported via taskStatusChannel in DownloadTask
func runDownloadTask(task *Task) *DownloadError {
	defer task.close()

	// create temporary folder in /tmp
	tmpDir, mkTmpDirErr := os.MkdirTemp("", "ddson")
	if mkTmpDirErr != nil {
		downloadErr := NewDownloadError(ErrCodeTempDirCreationFailed, mkTmpDirErr)
		slog.Error("Error creating temporary directory", "error", mkTmpDirErr)
		task.fail(downloadErr)
		return downloadErr
	}
	defer os.RemoveAll(tmpDir) // Clean up temporary directory
	slog.Info("saving temporary files", "dir", tmpDir)

	// create sub tasks
	task.subtasks = createSubtasks(task)
	totalSubTasks := len(task.subtasks)
	slog.Info("Created sub tasks", "count", totalSubTasks)

	startAllSubTasks(task)

	completedTasks := 0
	// this is for debugging purposes
	completedSubtaskMap := make(map[int]bool)

	for {
		subtaskID := <-task.subtaskDoneChan
		// for debugging purposes
		completedSubtaskMap[subtaskID] = true
		slog.Debug("completed subtasks", "IDs", completedSubtaskMap)

		slog.Info("subtask done", "subtaskID", subtaskID)
		completedTasks++
		if completedTasks == totalSubTasks {
			slog.Info("All subtasks completed")
			break
		}
	}

	// note: at this point, all subtasks have finished (either success or failure)

	downloadErr := task.taskStatus.Err
	if downloadErr != nil {
		slog.Error("Error executing sub tasks", "error", downloadErr)
		return downloadErr
	}
	slog.Info("All sub tasks executed", "count", totalSubTasks)

	completeFile, downloadErr := combine(task.subtasks, task.info.Size)
	if downloadErr != nil {
		slog.Error("Error combining files", "error", downloadErr)
		task.fail(downloadErr)
		return downloadErr
	}
	task.taskStatus.DownloadedFilePath = completeFile
	slog.Info("Combined file created", "file", completeFile)

	if task.info.Checksum != "" {
		slog.Info("Validating combined file", "file", completeFile, "checksum", task.info.Checksum)
		// TODO: nofity status as Validating
		if downloadErr != nil {
			slog.Error("Failed to send validation status", "error", downloadErr)
			task.fail(downloadErr)
			return downloadErr
		}
		downloadErr = validateFile(completeFile, task.info.Checksum)
		if downloadErr != nil {
			slog.Error("Error validating file", "error", downloadErr)
			task.fail(downloadErr)
			return downloadErr
		}
	} else {
		slog.Info("No checksum provided, skipping validation")
	}

	// TODO: update (broadcast) task state: DOWNLOAD_COMPLETED
	slog.Info("Download task completed successfully", "file", completeFile)
	return nil
}

func createSubtasks(task *Task) []*SubTask {
	totalSize := task.info.Size
	tmpDir := task.tmpFolder
	downloadUrl := task.info.DownloadUrl

	subtasks := make([]*SubTask, 0, totalSize/CHUNK_SIZE+1)
	i := 0
	for offset := uint64(0); offset < totalSize; offset += CHUNK_SIZE {
		downloadSize := CHUNK_SIZE
		if offset+downloadSize > totalSize {
			downloadSize = totalSize - offset
		}

		targetFile := fmt.Sprintf("%s/%d", tmpDir, offset)
		subTask := newDownloadSubTask(downloadUrl, i, offset, downloadSize, targetFile)
		i++
		subtasks = append(subtasks, subTask)
	}

	return subtasks
}

func startAllSubTasks(task *Task) {
	for _, subTask := range task.subtasks {
		go func(task *Task, subTask *SubTask) {
			slog.Debug("subtask started", "id", subTask.id)

			// TODO: real implementation
		}(task, subTask)
	}
}

func combine(subtasks []*SubTask, totalSize uint64) (string, *DownloadError) {
	// Create a new file to write the combined content
	combinedFile, err := os.CreateTemp("", "combined_")
	if err != nil {
		slog.Error("Error creating combined file", "error", err)
		return "", NewDownloadError(ErrCodeFileCreateFailed, err)
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

	var currentOffset uint64 = 0
	for _, subTask := range subtasks {
		if subTask.offset != currentOffset {
			slog.Error("Error: subtask offset mismatch", "got", subTask.offset, "want", currentOffset)
			return "", NewDownloadErrorWithMessage(ErrCodeFileSizeMismatch, fmt.Sprintf("subtask offset mismatch: got %d, want %d", subTask.offset, currentOffset))
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
			return "", NewDownloadError(ErrCodeFileOpenFailed, err)
		}
		defer file.Close()
		// Read the content and write it to the combined file
		_, err = io.Copy(combinedFile, file)
		if err != nil {
			slog.Error("Error writing to combined file", "error", err)
			return "", NewDownloadError(ErrCodeWriteFailed, err)
		}
		// Update the current offset
		currentOffset += subTask.downloadSize
	}

	if currentOffset != totalSize {
		slog.Error("Error: total size mismatch", "got", currentOffset, "want", totalSize)
		return "", NewDownloadErrorWithMessage(ErrCodeFileSizeMismatch, fmt.Sprintf("total size mismatch: got %d, want %d", currentOffset, totalSize))
	}

	return combinedFile.Name(), nil
}

func validateFile(file string, checksum string) *DownloadError {
	// calculate the checksum (sha256) of the file
	f, err := os.Open(file)
	if err != nil {
		return NewDownloadError(ErrCodeFileOpenFailed, err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		slog.Error("Error calculating checksum", "error", err)
		return NewDownloadError(ErrCodeReadFailed, err)
	}
	sum := hex.EncodeToString(hasher.Sum(nil))
	if sum != checksum {
		slog.Error("Checksum mismatch", "got", sum, "want", checksum)
		return NewDownloadErrorWithMessage(ErrCodeChecksumMismatch, fmt.Sprintf("checksum mismatch: got %s, want %s", sum, checksum))
	}
	return nil
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

func getDebugFinishedString(finishedSubtasks map[int]bool) string {
	var debugFinishedTaskBuffer = make([]byte, 0, len(finishedSubtasks)*3)
	debugFinishedTaskBuffer = append(debugFinishedTaskBuffer, '[')
	for i, finished := range finishedSubtasks {
		if finished {
			debugFinishedTaskBuffer = append(debugFinishedTaskBuffer, fmt.Sprintf("%d ", i)...)
		}
	}
	if len(debugFinishedTaskBuffer) > 1 && debugFinishedTaskBuffer[len(debugFinishedTaskBuffer)-1] == ' ' {
		debugFinishedTaskBuffer = debugFinishedTaskBuffer[:len(debugFinishedTaskBuffer)-1]
	}
	debugFinishedTaskBuffer = append(debugFinishedTaskBuffer, ']')
	return string(debugFinishedTaskBuffer)
}

func runSubtask(task *Task, subtask *SubTask) {
	slog.Debug("Executing subtask", "subtaskID", subtask.id, "offset", subtask.offset, "size", subtask.downloadSize, "targetFile", subtask.targetFile)

	for !task.abortFlag {
		agent := server.agentManager.GetIdleAgent(nil)
		if task.abortFlag {
			// Task was marked to quit while waiting for an agent
			slog.Info("Task quit flag set while waiting for agent, stopping subtask", "subtaskID", subTask.id)
			break
		}
		slog.Debug("Running subtask on agent", "subtaskID", subTask.id, "agentID", agent.GetID(), "agentAddr", agent.GetAddr())
		err := subTask.downloadChunk(&task.abortFlag, agent.GetAddr(), agent.GetID())
		server.agentManager.ReleaseAgent(agent, err == nil)

		if err == nil {
			// Success - report to error handler and exit loop
			break
		}

		// Let error handler decide what to do
		taskFailed := task.errorHandler.HandleSubtaskError(task, subTask, agent.GetID(), err)
		if taskFailed {
			// Error handler failed the task - stop retrying
			slog.Info("Task failed by error handler, stopping subtask", "subtaskID", subTask.id)
			subTask.err = err
			break
		}
		// Error handler says retry - loop continues
	}

	if task.abortFlag && subTask.err == nil {
		// Subtask was stopped by quit flag (another subtask failed the task)
		slog.Info("Subtask execution stopped by quit flag", "subtaskID", subTask.id)
		subTask.err = NewDownloadErrorWithMessage(ErrCodeDownloadCancelled, "cancelled by quit flag")
	}

	// Notify that the subtask finished
	slog.Info("Subtask execution finished, notifying task", "subtaskID", subTask.id, "error", subTask.err)
	finishChan <- subTask.id
	slog.Debug("Subtask execution finished, task notified", "subtaskID", subTask.id)
}

func (subTask *SubTask) downloadChunk(abortFlag *bool, addr string, agentID int) *DownloadError {
	downloadUrl, offset, downloadSize := subTask.downloadUrl, subTask.offset, subTask.downloadSize
	subtaskID := subTask.id
	slog.Info("Downloading chunk",
		"subtaskID", subtaskID,
		"url", downloadUrl,
		"offset", offset,
		"size", downloadSize,
		"agentID", agentID)

	// TODO: don't initialize a grpc agent for each download

	// Create a grpc request to the agent to ask for the download
	slog.Info("Connecting to agent", "subtaskID", subtaskID, "agentID", agentID, "address", addr)
	// Establish a connection to the server
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to connect to server", "subtaskID", subtaskID, "error", err)
		return NewGRPCError(ErrCodeGRPCConnectionFailed, err, agentID, "NewClient")
	}
	defer conn.Close()

	grpcClient := pb.NewDDSONServiceClientClient(conn)
	// Send the request to the agent
	stream, err := grpcClient.DownloadPart(context.Background(), &pb.DownloadPartRequest{
		Url:       downloadUrl,
		Offset:    offset,
		Size:      downloadSize,
		SubtaskId: int32(subtaskID),
		ClientId:  int32(agentID),
	})
	if err != nil {
		slog.Error("Error sending download request", "subtaskID", subtaskID, "error", err)
		return NewDownloadErrorFromError(ErrCodeGRPCConnectionFailed, err, agentID, "DownloadPart")
	}

	// Read the response from the agent
	targetFile := subTask.targetFile
	file, err := os.Create(targetFile)
	if err != nil {
		slog.Error("Error creating file", "subtaskID", subtaskID, "error", err)
		return NewDownloadError(ErrCodeFileCreateFailed, err)
	}
	defer file.Close()

	// Read the data from the stream and write it to the file
	slog.Info("Starting download for subtask", "subtaskID", subtaskID, "file", targetFile)
	var received int64 = 0
	currentState := pb.DownloadStatusType_PENDING
	for !*abortFlag {
		resp, err := stream.Recv()
		if err == io.EOF {
			slog.Debug("EOF received.", "subtaskID", subtaskID)
			break
		}
		if err != nil {
			slog.Error("Error receiving data", "subtaskID", subtaskID, "error", err)
			return NewDownloadErrorFromError(ErrCodeStreamRecvFailed, err, agentID, "DownloadPart.stream.Recv")
		}

		status := resp.GetStatus()
		if currentState != status {
			currentState = status
			slog.Debug("Subtask download status", "subtaskID", subtaskID, "status", status)
		}
		switch status {
		case pb.DownloadStatusType_DOWNLOADING:
			bytesDownloaded := resp.DownloadedBytes
			slog.Log(context.Background(), slog.LevelDebug-1, "Agent downloaded bytes", "subtaskID", subtaskID, "agentID", agentID, "bytes", bytesDownloaded)
			subTask.progressChan <- [2]int{agentID, int(bytesDownloaded)}

		case pb.DownloadStatusType_TRANSFERRING:
			// Write the data to the file
			dataSize := len(resp.GetData())
			slog.Debug("Writing data to file", "subtaskID", subtaskID, "size", dataSize)
			n, err := file.Write(resp.GetData())
			if err != nil {
				slog.Error("Error writing to file", "subtaskID", subtaskID, "error", err)
				return NewDownloadError(ErrCodeWriteFailed, err)
			}
			received += int64(n)
			slog.Debug("Data written to file", "subtaskID", subtaskID, "bytesWritten", n, "dataSize", dataSize, "totalReceived", received)

		default:
			slog.Error("Unexpected status", "subtaskID", subtaskID, "status", resp.GetStatus())
			return NewDownloadErrorWithMessage(ErrCodeUnexpectedStatus, fmt.Sprintf("unexpected status: %s", resp.GetStatus()))
		}
	}

	if *abortFlag {
		slog.Info("Download stopped by quit flag", "subtaskID", subtaskID)
		return NewDownloadErrorWithMessage(ErrCodeDownloadCancelled, "download stopped by quit flag")
	}
	if received != downloadSize {
		slog.Error("Error: received bytes mismatch", "subtaskID", subtaskID, "received", received, "expected", downloadSize)
		return NewDownloadErrorWithMessage(ErrCodeFileSizeMismatch, fmt.Sprintf("received %d bytes, expected %d bytes", received, downloadSize))
	}
	slog.Info("Download completed for subtask", "subtaskID", subtaskID, "file", targetFile)
	return nil
}
