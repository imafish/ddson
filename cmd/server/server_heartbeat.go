package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/imafish/ddson/internal/pb"
)

// TODO: try to use a long lived connection instead of heartbeats
func (s *server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	slog.Log(context.Background(), slog.LevelDebug-1, "Heartbeat received", "clientName", req.Name, "clientID", req.Id)
	id := int(req.Id)
	success := s.agentManager.HeartbeatReceived(id)

	if !success {
		slog.Warn("Heartbeat from unregistered client", "clientID", id, "clientName", req.Name)
		return &pb.HeartbeatResponse{
			Success: false,
			Message: fmt.Sprintf("client #%d not registered", id),
		}, nil
	}

	return &pb.HeartbeatResponse{
		Success: true,
		Message: "heartbeat received",
	}, nil
}
