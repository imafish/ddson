package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

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
	defer func() {
	}()

	for subTask.retryCount <= 3 && !*quitFlag {
		err := subTask.executeOnce(server, quitFlag)
		subTask.err = err
		if err == nil {
			break
		}
		slog.Error("Error executing subtask", "error", err, "subtaskID", subTask.id, "retryCount", subTask.retryCount)
		subTask.retryCount++
	}

	// if it reaches here, it means the subtask failed after retries
	if subTask.err != nil {
		slog.Error("Subtask failed after retries", "subtaskID", subTask.id, "error", subTask.err)
		*quitFlag = true
	}
	finishChan <- subTask.id
}

func (subTask *subTaskInfo) executeOnce(server *server, quitFlag *bool) error {
	// find a client to execute the subtask blocks until a client is available
	client := server.clients.getIdleClient()
	defer server.clients.releaseClient(client)
	slog.Info("Got idle client", "clientID", client.id, "subtaskID", subTask.id)
	if *quitFlag {
		slog.Info("Subtask execution stopped by quit flag", "subtaskID", subTask.id)
		return fmt.Errorf("subtask execution stopped by quit flag")
	}

	err := subTask.downloadChunk(client, quitFlag)
	if err != nil {
		slog.Error("Error downloading chunk", "error", err, "subtaskID", subTask.id, "retryCount", subTask.retryCount)
		return err
	}

	slog.Debug("Subtask completed successfully", "subtaskID", subTask.id, "file", subTask.targetFile)
	if client.errCount > 0 {
		client.errCount-- // decrement error count on success
	}
	return nil
}

func (subTask *subTaskInfo) downloadChunk(client *clientInfo, quitFlag *bool) error {
	downloadUrl, offset, downloadSize := subTask.downloadUrl, subTask.offset, subTask.downloadSize
	slog.Info("Downloading chunk",
		"subtaskID", subTask.id,
		"url", downloadUrl,
		"offset", offset,
		"size", downloadSize,
		"clientID", client.id)

	// TODO: don't initialize a grpc client for each download

	// Create a grpc request to the client to ask for the download
	clientAddr := fmt.Sprintf("%s:%d", client.addr, client.port)
	slog.Info("Connecting to client", "clientID", client.id, "address", clientAddr)
	// Establish a connection to the server
	conn, err := grpc.NewClient(clientAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to connect to server", "error", err)
	}
	defer conn.Close()

	grpcClient := pb.NewDDSONServiceClientClient(conn)
	if err != nil {
		slog.Error("Error creating gRPC client", "error", err)
		return err
	}
	// Send the request to the client
	stream, err := grpcClient.DownloadPart(context.Background(), &pb.DownloadPartRequest{
		Url:       downloadUrl,
		Offset:    offset,
		Size:      downloadSize,
		SubtaskId: int32(subTask.id),
		ClientId:  int32(subTask.assignedTo),
	})
	if err != nil {
		slog.Error("Error sending download request", "error", err)
		return err
	}
	defer stream.CloseSend()

	// Read the response from the client
	targetFile := subTask.targetFile
	file, err := os.Create(targetFile)
	if err != nil {
		slog.Error("Error creating file", "error", err)
		return err
	}
	defer file.Close()

	// Read the data from the stream and write it to the file
	slog.Info("Starting download for subtask", "subtaskID", subTask.id, "file", targetFile)
	var received int64 = 0
	currentState := pb.DownloadStatusType_PENDING
	for !*quitFlag {
		resp, err := stream.Recv()
		if err == io.EOF {
			slog.Debug("EOF received.", "subtaskID", subTask.id)
			break
		}
		if err != nil {
			slog.Error("Error receiving data", "error", err)
			return err
		}

		status := resp.GetStatus()
		switch status {
		case pb.DownloadStatusType_DOWNLOADING:
			if currentState != status {
				currentState = status
				slog.Info("Subtask download status", "subtaskID", subTask.id, "status", status)
			}
			clientID, bytesDownloaded := client.id, resp.DownloadedBytes
			slog.Debug("Client downloaded bytes", "clientID", clientID, "bytes", bytesDownloaded)
			subTask.progressChan <- [2]int{int(clientID), int(bytesDownloaded)}

		case pb.DownloadStatusType_TRANSFERRING:
			if currentState != status {
				currentState = status
				slog.Info("Subtask download status", "subtaskID", subTask.id, "status", status)
			}
			// Write the data to the file
			n, err := file.Write(resp.GetData())
			if err != nil {
				slog.Error("Error writing to file", "error", err)
				return err
			}
			received += int64(n)

		default:
			slog.Error("Unexpected status", "status", resp.GetStatus())
			return fmt.Errorf("unexpected status: %s", resp.GetStatus())
		}
	}

	if *quitFlag {
		slog.Info("Download stopped by quit flag", "subtaskID", subTask.id)
		return fmt.Errorf("download stopped by quit flag")
	}
	if received != downloadSize {
		slog.Error("Error: received bytes mismatch", "received", received, "expected", downloadSize)
		return fmt.Errorf("received %d bytes, expected %d bytes", received, downloadSize)
	}
	slog.Info("Download completed for subtask", "subtaskID", subTask.id, "file", targetFile)
	return nil
}
