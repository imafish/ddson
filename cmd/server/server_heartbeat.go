package main

import (
	"context"
	"fmt"
	"internal/pb"
	"log"
	"time"
)

func (s *server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	log.Printf("Received heartbeat from client %s (state: %v)", req.Name, req.State)
	client, exists := s.clients.getClientById(req.Id)
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
	client.state = req.State

	log.Printf("Updated client %s state to %v", req.Name, req.State)
	return &pb.HeartbeatResponse{
		Success: true,
		Message: "heartbeat received",
	}, nil
}

func (s *server) monitorClients() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		log.Printf("Checking dead clients...")
		now := time.Now()

		s.clients.mtx.Lock()
		for name, client := range s.clients.clients {
			if now.Sub(client.lastSeen) > 30*time.Second {
				log.Printf("Client %s heartbeat timeout, removing", name)
				delete(s.clients.clients, name)
				client.close()
			}
		}
		log.Printf("Finished checking dead clients.")
		s.clients.mtx.Unlock()
	}
}
