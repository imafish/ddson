package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"internal/agents"
	"internal/pb"
)

type subTaskInfo struct {
	downloadUrl  string
	id           int
	offset       int64
	downloadSize int64
	assignedTo   int
	targetFile   string
	err          error
	retryCount   int
	progressChan chan [2]int
}

func newSubTaskInfo(downloadUrl string, id int, offset int64, downloadSize int64, targetFile string, progressChan chan [2]int) *subTaskInfo {
	return &subTaskInfo{
		downloadUrl:  downloadUrl,
		id:           id,
		offset:       offset,
		downloadSize: downloadSize,
		assignedTo:   -1,
		targetFile:   targetFile,
		progressChan: progressChan,
	}
}

func (subTask *subTaskInfo) execute(task *taskInfo, server *server, finishChan chan int) {
	slog.Debug("Executing subtask", "subtaskID", subTask.id, "offset", subTask.offset, "size", subTask.downloadSize, "targetFile", subTask.targetFile)

	for !task.quitFlag {
		var lastAgentID int
		err := server.agentList.RunTask(func(agentInfo *agents.AgentInfo) error {
			lastAgentID = agentInfo.GetID()
			slog.Debug("Running subtask on agent", "subtaskID", subTask.id, "agentInfo", agentInfo)
			return subTask.downloadChunk(&task.quitFlag, agentInfo.GetAddr(), lastAgentID)
		})

		if err == nil {
			// Success - report to error handler and exit loop
			server.errorHandler.ReportAgentSuccess(lastAgentID)
			subTask.err = nil
			break
		}

		// Convert error to DownloadError and handle via error handler
		downloadErr := wrapAsDownloadError(err, subTask.downloadUrl, lastAgentID)
		subTask.err = downloadErr
		subTask.retryCount++

		// Let error handler decide what to do
		taskFailed := server.errorHandler.HandleSubtaskError(task, subTask, lastAgentID, downloadErr)
		if taskFailed {
			// Error handler failed the task - stop retrying
			slog.Info("Task failed by error handler, stopping subtask", "subtaskID", subTask.id)
			break
		}
		// Error handler says retry - loop continues
	}

	if task.quitFlag && subTask.err == nil {
		// Subtask was stopped by quit flag (another subtask failed the task)
		slog.Info("Subtask execution stopped by quit flag", "subtaskID", subTask.id)
		subTask.err = NewURLErrorWithMessage(ErrCodeDownloadCancelled, subTask.downloadUrl, "cancelled by quit flag")
	}

	// Notify that the subtask finished
	slog.Info("Subtask execution finished, notifying task", "subtaskID", subTask.id, "retryCount", subTask.retryCount, "error", subTask.err)
	finishChan <- subTask.id
	slog.Debug("Subtask execution finished, task notified", "subtaskID", subTask.id)
}

// wrapAsDownloadError converts a generic error to a DownloadError with appropriate error code
func wrapAsDownloadError(err error, url string, agentID int) *DownloadError {
	// If it's already a DownloadError, return it
	if de, ok := err.(*DownloadError); ok {
		return de
	}

	errMsg := err.Error()

	// Classify the error based on message patterns
	switch {
	case contains(errMsg, "connection refused", "connection reset", "no route to host"):
		return NewAgentError(ErrCodeGRPCConnectionFailed, agentID, err)
	case contains(errMsg, "context deadline exceeded", "timeout"):
		return NewAgentError(ErrCodeGRPCConnectionTimeout, agentID, err)
	case contains(errMsg, "EOF", "stream closed", "transport is closing"):
		return NewAgentError(ErrCodeStreamClosed, agentID, err)
	case contains(errMsg, "unavailable"):
		return NewAgentError(ErrCodeGRPCUnavailable, agentID, err)
	case contains(errMsg, "received", "expected", "bytes", "mismatch"):
		return NewURLErrorWithMessage(ErrCodeFileSizeMismatch, url, errMsg)
	case contains(errMsg, "unexpected status"):
		return NewURLErrorWithMessage(ErrCodeDownloadFailed, url, errMsg)
	case contains(errMsg, "quit flag", "cancelled"):
		return NewURLErrorWithMessage(ErrCodeDownloadCancelled, url, errMsg)
	default:
		// Default to generic download failed
		return NewURLError(ErrCodeDownloadFailed, url, err)
	}
}

// contains checks if s contains any of the substrings
func contains(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

func (subTask *subTaskInfo) downloadChunk(quitFlag *bool, addr string, agentID int) error {
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
		return NewAgentError(ErrCodeGRPCConnectionFailed, agentID, err)
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
		return NewAgentError(ErrCodeGRPCConnectionFailed, agentID, err)
	}

	// Read the response from the agent
	targetFile := subTask.targetFile
	file, err := os.Create(targetFile)
	if err != nil {
		slog.Error("Error creating file", "subtaskID", subtaskID, "error", err)
		return NewURLError(ErrCodeFileCreateFailed, downloadUrl, err)
	}
	defer file.Close()

	// Read the data from the stream and write it to the file
	slog.Info("Starting download for subtask", "subtaskID", subtaskID, "file", targetFile)
	var received int64 = 0
	currentState := pb.DownloadStatusType_PENDING
	for !*quitFlag {
		resp, err := stream.Recv()
		if err == io.EOF {
			slog.Debug("EOF received.", "subtaskID", subtaskID)
			break
		}
		if err != nil {
			slog.Error("Error receiving data", "subtaskID", subtaskID, "error", err)
			return NewAgentError(ErrCodeStreamRecvFailed, agentID, err)
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
				return NewURLError(ErrCodeWriteFailed, downloadUrl, err)
			}
			received += int64(n)
			slog.Debug("Data written to file", "subtaskID", subtaskID, "bytesWritten", n, "dataSize", dataSize, "totalReceived", received)

		default:
			slog.Error("Unexpected status", "subtaskID", subtaskID, "status", resp.GetStatus())
			return NewURLErrorWithMessage(ErrCodeDownloadFailed, downloadUrl, fmt.Sprintf("unexpected status: %s", resp.GetStatus()))
		}
	}

	if *quitFlag {
		slog.Info("Download stopped by quit flag", "subtaskID", subtaskID)
		return NewURLErrorWithMessage(ErrCodeDownloadCancelled, downloadUrl, "download stopped by quit flag")
	}
	if received != downloadSize {
		slog.Error("Error: received bytes mismatch", "subtaskID", subtaskID, "received", received, "expected", downloadSize)
		return NewURLErrorWithMessage(ErrCodeFileSizeMismatch, downloadUrl, fmt.Sprintf("received %d bytes, expected %d bytes", received, downloadSize))
	}
	slog.Info("Download completed for subtask", "subtaskID", subtaskID, "file", targetFile)
	return nil
}
