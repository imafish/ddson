package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/imafish/ddson/internal/downloadtask"
	"github.com/imafish/ddson/internal/httputil"
	"github.com/imafish/ddson/internal/pb"
)

func (s *server) Download(req *pb.DownloadRequest, stream pb.DDSONService_DownloadServer) error {
	slog.Info("Received download request", "url", req.GetUrl(), "agentID", req.GetClientId())

	// TODO: maybe later, only allow download from registered clients
	slog.Warn("NOT checking client id for now. implement later")
	agentID := 0
	downloadUrl := req.GetUrl()

	// Send initial status as PENDING
	err := stream.Send(&pb.DownloadStatus{
		Status:        pb.DownloadStatusType_PENDING,
		ClientCount:   int32(s.agentManager.GetAgentCount()),
		NumberInQueue: int32(len(s.taskManager.GetPendingTasks())),
	})
	if err != nil {
		slog.Error("Failed to send initial status", "error", err)
		return err
	}

	// Check in the database if the file is cached
	cached, err := s.persistency.GetPersistedFile(req.GetUrl(), req.GetChecksum())
	if err != nil {
		slog.Error("Failed to check cached file", "url", req.GetUrl(), "error", err)
	} else if cached != "" {
		slog.Info("File is cached, sending cached file", "url", req.GetUrl(), "cachedPath", cached)
		// Send file content from cache
		// transferFileData is from distributed_download.go, consider moving this method to a common place
		return transferFileData(stream, cached)
	} else {
		slog.Info("File is not cached, proceed with download", "url", req.GetUrl())
	}

	// Get credentials from .netrc
	login, password, err := httputil.GetDataFromNetrc(downloadUrl)
	if err != nil {
		slog.Error("Error getting credentials from .netrc", "error", err)
		return err
	}

	// Check if server supports partial downloads
	supportsPartial, totalSize, err := httputil.CheckPartialDownloadSupport(downloadUrl, http.DefaultClient, login, password)
	if err != nil {
		slog.Error("Error checking partial download support", "error", err)
		return err
	}
	if !supportsPartial {
		slog.Warn("Server does not support partial downloads, downloading the whole file")
		err = fmt.Errorf("server does not support partial downloads")
		return err
	}

	// Create a task and add it to task list
	task := downloadtask.NewTask(req.GetUrl(), req.GetChecksum(), uint64(totalSize), stream, 0, agentID, s.agentManager)
	defer task.Cleanup()
	_ = s.taskManager.AddTask(task)

	// periodically update the status (using select?)
	taskStatusChan := task.GetTaskStatusChannel()
	for {
		taskStatus := <-taskStatusChan
		err = reportStatus(&taskStatus, stream)
		if err != nil {
			slog.Error("Failed to report status to callin client", "error", err)
			break
		}
		if taskStatus.Status == downloadtask.TaskStatusCompleted || taskStatus.Status == downloadtask.TaskStatusFailed {
			break
		}
	}

	finalStatus := task.GetTaskStatus()
	if finalStatus.Status == downloadtask.TaskStatusFailed {
		slog.Error("Task is done, error in task", "error", finalStatus.Err)
		return finalStatus.Err
	}

	// TODO: move these to a background goroutine.
	// save the downloaded file to persistency
	if finalStatus.DownloadedFilePath != "" {
		slog.Info("Saving downloaded file to persistency", "path", finalStatus.DownloadedFilePath)
		err = s.persistency.AddDownloadedFile(req.GetUrl(), finalStatus.DownloadedFilePath, req.GetChecksum())
		if err != nil {
			slog.Error("Failed to save downloaded file", "url", req.GetUrl(), "error", err)
		}
	} else {
		slog.Debug("No need to update persistency")
	}

	// cleanup persistency
	slog.Debug("Cleaning up persistency")
	err = s.persistency.Cleanup(time.Second*60*60*24*16, 100*1024*1024*1024, 200*1024*1024*1024) // 16 days, 100 GB, 200 GB
	if err != nil {
		slog.Warn("Failed to cleanup persistency", "error", err)
	}

	slog.Info("Task is done", "url", req.GetUrl())
	return nil
}

func reportStatus(status *downloadtask.TaskStatus, stream pb.DDSONService_DownloadServer) error {
	// Map TaskStatusEnum to DownloadStatusType
	var pbStatus pb.DownloadStatusType
	switch status.Status {
	case downloadtask.TaskStatusNew, downloadtask.TaskStatusPending:
		pbStatus = pb.DownloadStatusType_PENDING
	case downloadtask.TaskStatusPreparing, downloadtask.TaskStatusDownloading:
		pbStatus = pb.DownloadStatusType_DOWNLOADING
	case downloadtask.TaskStatusValidating, downloadtask.TaskStatusPostProcessing:
		pbStatus = pb.DownloadStatusType_VALIDATING
	case downloadtask.TaskStatusTransferring, downloadtask.TaskStatusDownloadCompleted:
		pbStatus = pb.DownloadStatusType_TRANSFERRING
	case downloadtask.TaskStatusCompleted:
		pbStatus = pb.DownloadStatusType_TRANSFERRING
	case downloadtask.TaskStatusFailed:
		pbStatus = pb.DownloadStatusType_ERROR
	default:
		pbStatus = pb.DownloadStatusType_PENDING
	}

	resp := &pb.DownloadStatus{
		Status: pbStatus,
		Speed:  int32(status.DownloadSpeed),
	}

	// Add context-specific fields
	switch status.Status {
	case downloadtask.TaskStatusPending:
		resp.NumberInQueue = int32(status.PositionInQueue)
	case downloadtask.TaskStatusFailed:
		if status.Err != nil {
			resp.Message = status.Err.Error()
		}
	}

	return stream.Send(resp)
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
