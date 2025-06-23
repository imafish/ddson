package main

import (
	"log/slog"

	"internal/pb"
)

func (s *server) Download(req *pb.DownloadRequest, stream pb.DDSONService_DownloadServer) error {
	slog.Info("Received download request", "url", req.GetUrl(), "agentID", req.GetClientId())

	// TODO: maybe later, only allow download from registered clients
	slog.Warn("NOT checking client id for now. implement later")
	agentID := 0

	// Send initial status as PENDING
	err := stream.Send(&pb.DownloadStatus{
		Status:        pb.DownloadStatusType_PENDING,
		ClientCount:   int32(s.agentList.Count()),
		NumberInQueue: int32(s.taskList.size()),
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
		slog.Info("File is not cached, proceeding with download", "url", req.GetUrl())
	}

	// Create a task and add it to task list
	taskInfo := s.taskList.addTask(req.GetUrl(), req.GetChecksum(), stream, agentID)

	// wait for the task to complete
	// TODO: periodically update the status (using select?)
	<-taskInfo.done

	if taskInfo.err != nil {
		slog.Error("Task is done, error in task", "error", taskInfo.err)
		return taskInfo.err
	}

	// save the downloaded file to persistency
	if taskInfo.downloadedFile != "" {
		slog.Info("Saving downloaded file to persistency", "path", taskInfo.downloadedFile)
		err = s.persistency.NewDownloadedFile(req.GetUrl(), taskInfo.downloadedFile, req.GetChecksum())
		if err != nil {
			slog.Error("Failed to save downloaded file", "url", req.GetUrl(), "error", err)
		}
	}

	slog.Info("Task is done", "url", req.GetUrl())
	return nil
}
