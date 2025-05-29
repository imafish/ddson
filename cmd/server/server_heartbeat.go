package main

import (
	"context"
	"fmt"
	"internal/pb"
	"log/slog"
	"time"
)

func (s *server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	slog.Debug("Received heartbeat", "clientName", req.Name, "clientID", req.Id)
	client, exists := s.clients.getClientById(int(req.Id))
	if !exists {
		return &pb.HeartbeatResponse{
			Success: false,
			Message: fmt.Sprintf("client #%d not registered", req.Id),
		}, nil
	}

	if client.name != req.Name {
		return &pb.HeartbeatResponse{
			Success: false,
			Message: fmt.Sprintf("client name mismatch: expected %s, got %s", client.name, req.Name),
		}, nil
	}

	client.lastSeen = time.Now()

	return &pb.HeartbeatResponse{
		Success: true,
		Message: "heartbeat received",
	}, nil
}

func (s *server) monitorClients() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		slog.Debug("Checking dead clients...")
		now := time.Now()

		s.clients.mtx.Lock()
		for id, client := range s.clients.clients {
			if now.Sub(client.lastSeen) > 20*time.Second {
				slog.Warn("Client heartbeat timeout, removing", "clientID", id, "name", client.name, "address", client.addr)
				s.clients.removeAndCloseClientNoLock(id)
			}
			if client.errCount > 5 {
				slog.Warn("Client error count exceeded, removing", "clientID", id, "errorCount", client.errCount)
				s.clients.removeAndCloseClientNoLock(id)
				s.clients.banClientNoLock(client, 300) // Ban for 5 minutes
			}
		}
		slog.Debug("Finished checking dead clients.")
		s.clients.mtx.Unlock()
	}
}
