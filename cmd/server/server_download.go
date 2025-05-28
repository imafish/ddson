package main

import (
	"log/slog"

	"internal/pb"
)

func (s *server) Download(req *pb.DownloadRequest, stream pb.DDSONService_DownloadServer) error {
	slog.Info("Received download request", "url", req.GetUrl(), "clientID", req.GetClientId())

	// the download request must be from a client in the list
	// TODO: matching client with name is not accurate, improve later
	slog.Warn("NOT checking client id for now. implement later")
	clientId := 0
	// _, exists := s.clients.getClientById(int(req.GetClientId()))
	// if !exists {
	// 	log.Printf("Client #%d not found in the list", req.GetClientId())
	// 	err := fmt.Errorf("client #%d not found in the list", req.GetClientId())
	// 	return err
	// }

	// Send initial status as PENDING
	err := stream.Send(&pb.DownloadStatus{
		Status:        pb.DownloadStatusType_PENDING,
		ClientCount:   int32(len(s.clients.clients)),
		NumberInQueue: int32(s.taskList.size()),
	})
	if err != nil {
		slog.Error("Failed to send initial status", "error", err)
		return err
	}

	// Create a task and add it to task list
	taskInfo := s.taskList.addTask(req.GetUrl(), req.GetChecksum(), stream, clientId)

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
