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

func (subTask *subTaskInfo) execute(server *server, quitFlag *bool, finishChan chan int) {
	slog.Debug("Executing subtask", "subtaskID", subTask.id, "offset", subTask.offset, "size", subTask.downloadSize, "targetFile", subTask.targetFile)

	for subTask.retryCount <= 3 && !*quitFlag {
		err := server.agentList.RunTask(func(agentInfo *agents.AgentInfo) error {
			slog.Debug("Running subtask on agent", "subtaskID", subTask.id, "agentInfo", agentInfo)
			return subTask.downloadChunk(quitFlag, agentInfo.GetAddr(), agentInfo.GetID())
		})
		subTask.err = err
		if err == nil {
			break
		}
		subTask.retryCount++
		slog.Error("Error executing subtask", "error", err, "subtaskID", subTask.id, "retryCount", subTask.retryCount)
	}

	if *quitFlag {
		// if we reach here, it means the subtask was stopped by the quit flag
		slog.Info("Subtask execution stopped by quit flag", "subtaskID", subTask.id)
	} else if subTask.err != nil {
		// if we reach here, it means the subtask failed after retries
		slog.Error("Subtask failed after retries", "subtaskID", subTask.id, "error", subTask.err)
	}

	// notify that the subtask either way
	slog.Info("Subtask execution finished, notifying task", "subtaskID", subTask.id, "retryCount", subTask.retryCount, "error", subTask.err)
	finishChan <- subTask.id
	slog.Debug("Subtask execution finished, task notified", "subtaskID", subTask.id)
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
	}
	defer conn.Close()

	grpcClient := pb.NewDDSONServiceClientClient(conn)
	if err != nil {
		slog.Error("Error creating gRPC client", "subtaskID", subtaskID, "error", err)
		return err
	}
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
		return err
	}

	// Read the response from the agent
	targetFile := subTask.targetFile
	file, err := os.Create(targetFile)
	if err != nil {
		slog.Error("Error creating file", "subtaskID", subtaskID, "error", err)
		return err
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
			return err
		}

		status := resp.GetStatus()
		if currentState != status {
			currentState = status
			slog.Debug("Subtask download status", "subtaskID", subtaskID, "status", status)
		}
		switch status {
		case pb.DownloadStatusType_DOWNLOADING:
			bytesDownloaded := resp.DownloadedBytes
			slog.Debug("Agent downloaded bytes", "subtaskID", subtaskID, "agentID", agentID, "bytes", bytesDownloaded)
			subTask.progressChan <- [2]int{agentID, int(bytesDownloaded)}

		case pb.DownloadStatusType_TRANSFERRING:
			// Write the data to the file
			dataSize := len(resp.GetData())
			slog.Debug("Writing data to file", "subtaskID", subtaskID, "size", dataSize)
			n, err := file.Write(resp.GetData())
			if err != nil {
				slog.Error("Error writing to file", "subtaskID", subtaskID, "error", err)
				return err
			}
			received += int64(n)
			slog.Debug("Data written to file", "subtaskID", subtaskID, "bytesWritten", n, "dataSize", dataSize, "totalReceived", received)

		default:
			slog.Error("Unexpected status", "subtaskID", subtaskID, "status", resp.GetStatus())
			return fmt.Errorf("unexpected status: %s", resp.GetStatus())
		}
	}

	if *quitFlag {
		slog.Info("Download stopped by quit flag", "subtaskID", subtaskID)
		return fmt.Errorf("download stopped by quit flag")
	}
	if received != downloadSize {
		slog.Error("Error: received bytes mismatch", "subtaskID", subtaskID, "received", received, "expected", downloadSize)
		return fmt.Errorf("received %d bytes, expected %d bytes", received, downloadSize)
	}
	slog.Info("Download completed for subtask", "subtaskID", subtaskID, "file", targetFile)
	return nil
}
