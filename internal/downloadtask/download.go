package downloadtask

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"sort"

	"github.com/imafish/ddson/internal/agents"
	"github.com/imafish/ddson/internal/common"
	"github.com/imafish/ddson/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const chunkSize = uint64(10 * 1024 * 1024) // 10 MB

// run the download task.
// execution status is reported via taskStatusChannel in DownloadTask
func runDownloadTask(task *Task) *DownloadError {
	task.updateAndSendTaskStatus("Status", TaskStatusPreparing)

	// create temporary folder in /tmp
	tmpDir, mkTmpDirErr := task.fs.MkdirTemp("", "ddson")
	if mkTmpDirErr != nil {
		downloadErr := NewDownloadError(ErrCodeTempDirCreationFailed, mkTmpDirErr)
		logger.Error("Error creating temporary directory", "error", mkTmpDirErr)
		task.fail(downloadErr)
		return downloadErr
	}
	task.tmpFolder = tmpDir
	logger.Info("temporary directory", "dir", tmpDir)

	// create sub tasks
	task.subtasks = createSubtasks(task)
	totalSubTasks := len(task.subtasks)
	logger.Info("Created sub tasks", "count", totalSubTasks)

	task.updateAndSendTaskStatus("Status", TaskStatusDownloading)
	startAllSubTasks(task)

	completedTasks := 0
	// this is for debugging purposes
	completedSubtaskMap := make(map[int]bool)

	for {
		subtaskID := <-task.subtaskDoneChan
		// for debugging purposes
		completedSubtaskMap[subtaskID] = true
		logger.Debug("completed subtasks", "IDs", getDebugFinishedString(completedSubtaskMap))

		logger.Info("subtask done", "subtaskID", subtaskID)
		completedTasks++
		if completedTasks == totalSubTasks {
			logger.Info("All subtasks completed")
			break
		}
	}

	// note: at this point, all subtasks have finished (either success or failure)

	downloadErr := task.taskStatus.Err
	if downloadErr != nil {
		logger.Error("Error executing subtasks", "error", downloadErr)
		task.fail(downloadErr)
		return downloadErr
	}
	logger.Info("All subtasks executed", "count", totalSubTasks)

	task.updateAndSendTaskStatus("Status", TaskStatusPostProcessing)
	completeFile, downloadErr := combine(task.fs, task.subtasks, task.info.Size)
	if downloadErr != nil {
		logger.Error("Error combining files", "error", downloadErr)
		task.fail(downloadErr)
		return downloadErr
	}
	task.taskStatus.DownloadedFilePath = completeFile
	logger.Info("Combined file created", "file", completeFile)

	if task.info.Checksum != "" {
		logger.Info("Validating combined file", "file", completeFile, "checksum", task.info.Checksum)
		// TODO: nofity status as Validating
		task.updateAndSendTaskStatus("Status", TaskStatusValidating)
		downloadErr = validateFile(task.fs, completeFile, task.info.Checksum)
		if downloadErr != nil {
			logger.Error("Error validating file", "error", downloadErr)
			task.fail(downloadErr)
			return downloadErr
		}
	} else {
		logger.Info("No checksum provided, skipping validation")
	}

	task.updateAndSendTaskStatus("Status", TaskStatusDownloadCompleted)
	logger.Info("Download task completed successfully", "file", completeFile)
	return nil
}

func createSubtasks(task *Task) []*subTask {
	totalSize := task.info.Size
	tmpDir := task.tmpFolder
	downloadUrl := task.info.DownloadUrl

	subtasks := make([]*subTask, 0, totalSize/chunkSize+1)
	i := 0
	for offset := uint64(0); offset < totalSize; offset += chunkSize {
		downloadSize := chunkSize
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
		executor := task.subtaskExecutor
		if executor == nil {
			executor = defaultSubtaskExecutor{}
		}
		go executor.Run(task, subTask)
	}
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

func combine(fs common.FileSystem, subtasks []*subTask, totalSize uint64) (string, *DownloadError) {
	// Create a new file to write the combined content
	combinedFile, err := fs.CreateTemp("", "combined_")
	if err != nil {
		logger.Error("Error creating combined file", "error", err)
		return "", NewDownloadError(ErrCodeFileCreateFailed, err)
	}
	defer combinedFile.Close()
	logger.Info("Combining sub tasks into file", "file", combinedFile.Name())

	// sort the completed sub tasks by offset
	sort.Slice(subtasks, func(i, j int) bool {
		return subtasks[i].offset < subtasks[j].offset
	})

	// print sorted sub tasks
	for _, subTask := range subtasks {
		logger.Debug("Sub task info", "subtaskID", subTask.id, "offset", subTask.offset, "size", subTask.downloadSize, "file", subTask.targetFile)
	}

	var currentOffset uint64 = 0
	for _, subTask := range subtasks {
		if subTask.offset != currentOffset {
			logger.Error("Error: subtask offset mismatch", "got", subTask.offset, "want", currentOffset)
			return "", NewDownloadErrorWithMessage(ErrCodeFileSizeMismatch, fmt.Sprintf("subtask offset mismatch: got %d, want %d", subTask.offset, currentOffset))
		}
		currentOffset += subTask.downloadSize
	}

	currentOffset = 0
	for _, subTask := range subtasks {
		logger.Debug("Combining sub task", "subtaskID", subTask.id, "offset", subTask.offset, "size", subTask.downloadSize)
		// Open the sub task file
		file, err := fs.Open(subTask.targetFile)
		if err != nil {
			logger.Error("Error opening sub task file", "error", err)
			return "", NewDownloadError(ErrCodeFileOpenFailed, err)
		}
		defer file.Close()
		// Read the content and write it to the combined file
		_, err = io.Copy(combinedFile, file)
		if err != nil {
			logger.Error("Error writing to combined file", "error", err)
			return "", NewDownloadError(ErrCodeWriteFailed, err)
		}
		// Update the current offset
		currentOffset += subTask.downloadSize
	}

	if currentOffset != totalSize {
		logger.Error("Error: total size mismatch", "got", currentOffset, "want", totalSize)
		return "", NewDownloadErrorWithMessage(ErrCodeFileSizeMismatch, fmt.Sprintf("total size mismatch: got %d, want %d", currentOffset, totalSize))
	}

	return combinedFile.Name(), nil
}

func validateFile(fs common.FileSystem, file string, checksum string) *DownloadError {
	// calculate the checksum (sha256) of the file
	f, err := fs.Open(file)
	if err != nil {
		return NewDownloadError(ErrCodeFileOpenFailed, err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		logger.Error("Error calculating checksum", "error", err)
		return NewDownloadError(ErrCodeReadFailed, err)
	}
	sum := hex.EncodeToString(hasher.Sum(nil))
	if sum != checksum {
		logger.Error("Checksum mismatch", "got", sum, "want", checksum)
		return NewDownloadErrorWithMessage(ErrCodeChecksumMismatch, fmt.Sprintf("checksum mismatch: got %s, want %s", sum, checksum))
	}
	return nil
}

func runSubtask(task *Task, subtask *subTask) {
	logger.Debug("Executing subtask", "subtaskID", subtask.id, "offset", subtask.offset, "size", subtask.downloadSize, "targetFile", subtask.targetFile)
	agentManager := task.agentManager
	collector := task.downloadStatusCollector

	for !task.abortFlag {
		agent := agentManager.GetIdleAgent(nil)
		if task.abortFlag {
			// Task was marked to quit while waiting for an agent
			logger.Info("Task quit flag set while waiting for agent, stopping subtask", "subtaskID", subtask.id)
			break
		}

		logger.Debug("Running subtask on agent", "subtaskID", subtask.id, "agentID", agent.GetID(), "agentAddr", agent.GetAddr())

		// Notify collector about subtask start (OnSubtaskError already wiped data if this is a retry)
		collector.onSubtaskStart(subtask.id, agent.GetID())

		err := downloadChunk(task, subtask, agent)
		agentManager.ReleaseAgent(agent, err == nil)

		if err == nil {
			// Notify collector about successful completion
			collector.onSubtaskCompleted(subtask.id, subtask.targetFile)
			break
		}

		// Notify collector about error
		collector.onSubtaskError(subtask.id, err)

		// Let error handler decide what to do
		taskFailed := task.errorHandler.handleSubtaskError(task, subtask, agent.GetID(), err)
		if taskFailed {
			// Error handler failed the task - stop retrying
			logger.Info("Task failed by error handler, stopping subtask", "subtaskID", subtask.id)
			subtask.err = err
			break
		}
		// Error handler says retry - loop continues
	}

	if task.abortFlag && subtask.err == nil {
		// Subtask was stopped by quit flag (another subtask failed the task)
		logger.Info("Subtask execution stopped by quit flag", "subtaskID", subtask.id)
		subtask.err = NewDownloadErrorWithMessage(ErrCodeDownloadCancelled, "cancelled by quit flag")
	}

	// Notify that the subtask finished
	logger.Info("Subtask execution finished, notifying task", "subtaskID", subtask.id, "error", subtask.err)
	task.subtaskDoneChan <- subtask.id
	logger.Debug("Subtask execution finished, task notified", "subtaskID", subtask.id)
}

func downloadChunk(task *Task, subtask *subTask, agent *agents.AgentInfo) *DownloadError {
	downloadUrl, offset, downloadSize := subtask.downloadUrl, subtask.offset, subtask.downloadSize
	agentEndpoint, agentID := agent.GetAddr(), agent.GetID()
	subtaskID := subtask.id
	logger.Info("Downloading chunk",
		"subtaskID", subtaskID,
		"url", downloadUrl,
		"offset", offset,
		"size", downloadSize,
		"agentID", agentID)

	// TODO: don't initialize a grpc agent for each download

	// Create a grpc request to the agent to ask for the download
	logger.Info("Connecting to agent", "subtaskID", subtaskID, "agentID", agentID, "address", agentEndpoint)
	// Establish a connection to the server
	conn, err := grpc.NewClient(agentEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("failed to connect to server", "subtaskID", subtaskID, "error", err)
		return NewGRPCError(ErrCodeGRPCConnectionFailed, err, agentID, "NewClient")
	}
	defer conn.Close()

	grpcClient := pb.NewDDSONServiceClientClient(conn)
	// Send the request to the agent
	stream, err := grpcClient.DownloadPart(context.Background(), &pb.DownloadPartRequest{
		Url: downloadUrl,
		// TODO: Offset and Size should be uint64 in protobuf
		Offset:    int64(offset),
		Size:      int64(downloadSize),
		SubtaskId: int32(subtaskID),
		ClientId:  int32(agentID),
	})
	if err != nil {
		logger.Error("Error sending download request", "subtaskID", subtaskID, "error", err)
		return NewDownloadErrorFromError(ErrCodeGRPCConnectionFailed, err, agentID, "DownloadPart")
	}

	// Read the response from the agent
	targetFile := subtask.targetFile
	file, err := task.fs.Create(targetFile)
	if err != nil {
		logger.Error("Error creating file", "subtaskID", subtaskID, "error", err)
		return NewDownloadError(ErrCodeFileCreateFailed, err)
	}
	defer file.Close()

	// Read the data from the stream and write it to the file
	logger.Info("Starting download for subtask", "subtaskID", subtaskID, "file", targetFile)
	var received uint64 = 0
	currentState := pb.DownloadStatusType_PENDING
	for !task.abortFlag {
		resp, err := stream.Recv()
		if err == io.EOF {
			logger.Debug("EOF received.", "subtaskID", subtaskID)
			break
		}
		if err != nil {
			logger.Error("Error receiving data", "subtaskID", subtaskID, "error", err)
			return NewDownloadErrorFromError(ErrCodeStreamRecvFailed, err, agentID, "DownloadPart.stream.Recv")
		}

		status := resp.GetStatus()
		if currentState != status {
			currentState = status
			logger.Debug("Subtask download status", "subtaskID", subtaskID, "status", status)
		}

		switch status {
		case pb.DownloadStatusType_DOWNLOADING:
			bytesDownloaded := resp.DownloadedBytes
			logger.Log(context.Background(), slog.LevelDebug-1, "Agent downloaded bytes", "subtaskID", subtaskID, "agentID", agentID, "bytes", bytesDownloaded)
			task.downloadStatusCollector.onSubtaskProgress(subtaskID, bytesDownloaded)

		case pb.DownloadStatusType_TRANSFERRING:
			// Write the data to the file
			dataSize := len(resp.GetData())
			logger.Debug("Writing data to file", "subtaskID", subtaskID, "size", dataSize)
			n, err := file.Write(resp.GetData())
			if err != nil {
				logger.Error("Error writing to file", "subtaskID", subtaskID, "error", err)
				return NewDownloadError(ErrCodeWriteFailed, err)
			}
			received += uint64(n)
			logger.Debug("Data written to file", "subtaskID", subtaskID, "bytesWritten", n, "dataSize", dataSize, "totalReceived", received)

		default:
			logger.Error("Unexpected status", "subtaskID", subtaskID, "status", resp.GetStatus())
			return NewDownloadErrorWithMessage(ErrCodeUnexpectedStatus, fmt.Sprintf("unexpected status: %s", resp.GetStatus()))
		}
	}

	if task.abortFlag {
		logger.Info("Download stopped by quit flag", "subtaskID", subtaskID)
		return NewDownloadErrorWithMessage(ErrCodeDownloadCancelled, "download stopped by quit flag")
	}
	if uint64(received) != downloadSize {
		logger.Error("Error: received bytes mismatch", "subtaskID", subtaskID, "received", received, "expected", downloadSize)
		return NewDownloadErrorWithMessage(ErrCodeFileSizeMismatch, fmt.Sprintf("received %d bytes, expected %d bytes", received, downloadSize))
	}
	logger.Info("Download completed for subtask", "subtaskID", subtaskID, "file", targetFile)
	return nil
}
