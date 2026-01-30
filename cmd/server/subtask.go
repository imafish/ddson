package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/imafish/ddson/internal/pb"
)

type subTaskInfo struct {
	downloadUrl  string
	id           int
	offset       int64
	downloadSize int64
	targetFile   string
	err          *DownloadError
	progressChan chan [2]int
}

func newSubTaskInfo(downloadUrl string, id int, offset int64, downloadSize int64, targetFile string, progressChan chan [2]int) *subTaskInfo {
	return &subTaskInfo{
		downloadUrl:  downloadUrl,
		id:           id,
		offset:       offset,
		downloadSize: downloadSize,
		targetFile:   targetFile,
		err:          nil,
		progressChan: progressChan,
	}
}

// TODO: execute should not be a method of subtask
// TODO: quitFlag should be a channel, rename to taskAbortChan
func (subTask *subTaskInfo) execute(task *taskInfo, server *server, finishChan chan int) {
	slog.Debug("Executing subtask", "subtaskID", subTask.id, "offset", subTask.offset, "size", subTask.downloadSize, "targetFile", subTask.targetFile)

	for !task.quitFlag {
		agent := server.agentManager.GetIdleAgent(nil)
		if task.quitFlag {
			// Task was marked to quit while waiting for an agent
			slog.Info("Task quit flag set while waiting for agent, stopping subtask", "subtaskID", subTask.id)
			break
		}
		slog.Debug("Running subtask on agent", "subtaskID", subTask.id, "agentID", agent.GetID(), "agentAddr", agent.GetAddr())
		err := subTask.downloadChunk(&task.quitFlag, agent.GetAddr(), agent.GetID())
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

	if task.quitFlag && subTask.err == nil {
		// Subtask was stopped by quit flag (another subtask failed the task)
		slog.Info("Subtask execution stopped by quit flag", "subtaskID", subTask.id)
		subTask.err = NewDownloadErrorWithMessage(subTask.downloadUrl, ErrCodeDownloadCancelled, "cancelled by quit flag")
	}

	// Notify that the subtask finished
	slog.Info("Subtask execution finished, notifying task", "subtaskID", subTask.id, "error", subTask.err)
	finishChan <- subTask.id
	slog.Debug("Subtask execution finished, task notified", "subtaskID", subTask.id)
}

func (subTask *subTaskInfo) downloadChunk(quitFlag *bool, addr string, agentID int) *DownloadError {
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
		return NewGRPCError(downloadUrl, ErrCodeGRPCConnectionFailed, err, agentID, "NewClient")
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
		return NewDownloadErrorFromError(downloadUrl, ErrCodeGRPCConnectionFailed, err, agentID, "DownloadPart")
	}

	// Read the response from the agent
	targetFile := subTask.targetFile
	file, err := os.Create(targetFile)
	if err != nil {
		slog.Error("Error creating file", "subtaskID", subtaskID, "error", err)
		return NewDownloadError(downloadUrl, ErrCodeFileCreateFailed, err)
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
			return NewDownloadErrorFromError(downloadUrl, ErrCodeStreamRecvFailed, err, agentID, "DownloadPart.stream.Recv")
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
				return NewDownloadError(downloadUrl, ErrCodeWriteFailed, err)
			}
			received += int64(n)
			slog.Debug("Data written to file", "subtaskID", subtaskID, "bytesWritten", n, "dataSize", dataSize, "totalReceived", received)

		default:
			slog.Error("Unexpected status", "subtaskID", subtaskID, "status", resp.GetStatus())
			return NewDownloadErrorWithMessage(downloadUrl, ErrCodeUnexpectedStatus, fmt.Sprintf("unexpected status: %s", resp.GetStatus()))
		}
	}

	if *quitFlag {
		slog.Info("Download stopped by quit flag", "subtaskID", subtaskID)
		return NewDownloadErrorWithMessage(downloadUrl, ErrCodeDownloadCancelled, "download stopped by quit flag")
	}
	if received != downloadSize {
		slog.Error("Error: received bytes mismatch", "subtaskID", subtaskID, "received", received, "expected", downloadSize)
		return NewDownloadErrorWithMessage(downloadUrl, ErrCodeFileSizeMismatch, fmt.Sprintf("received %d bytes, expected %d bytes", received, downloadSize))
	}
	slog.Info("Download completed for subtask", "subtaskID", subtaskID, "file", targetFile)
	return nil
}
