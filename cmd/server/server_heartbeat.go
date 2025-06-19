package main

import (
	"context"
	"fmt"
	"internal/pb"
	"log/slog"
	"time"
)

// TODO: try to use a long lived connection instead of heartbeats
func (s *server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	slog.Log(context.Background(), slog.LevelDebug-1, "Heartbeat received", "clientName", req.Name, "clientID", req.Id)
	id := int(req.Id)
	agent := s.agentList.GetAgentByID(id)
	if agent == nil {
		slog.Warn("Heartbeat from unregistered client", "clientID", id, "clientName", req.Name)
		return &pb.HeartbeatResponse{
			Success: false,
			Message: fmt.Sprintf("client #%d not registered", id),
		}, nil
	}

	s.heartbeatTimers[id].Reset(20 * time.Second) // Reset the heartbeat timer for this client

	return &pb.HeartbeatResponse{
		Success: true,
		Message: "heartbeat received",
	}, nil
}
