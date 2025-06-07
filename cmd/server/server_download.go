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

	// Create a task and add it to task list
	taskInfo := s.taskList.addTask(req.GetUrl(), req.GetChecksum(), stream, agentID)

	// wait for the task to complete
	// TODO: periodically update the status (using select?)
	<-taskInfo.done

	// send file content
	if taskInfo.err != nil {
		slog.Error("Task is done, error in task", "error", taskInfo.err)
		return taskInfo.err
	}

	slog.Info("Task is done, no error found", "url", req.GetUrl())
	return nil
}
