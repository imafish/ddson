package main

import (
	"context"
	"internal/pb"
	"log"
	"time"
)

func (s *server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	log.Printf("Received heartbeat from client %s (state: %v)", req.Name, req.State)
	s.clientsLock.Lock()
	defer s.clientsLock.Unlock()
	client, exists := s.clients[req.Name]

	if !exists {
		return &pb.HeartbeatResponse{
			Success: false,
			Message: "client not registered",
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
		s.clientsLock.Lock()
		log.Printf("Checking client heartbeats...")
		now := time.Now()
		for name, client := range s.clients {
			if now.Sub(client.lastSeen) > 30*time.Second {
				log.Printf("Client %s heartbeat timeout, removing", name)
				delete(s.clients, name)
				client.close()
			}
		}
		log.Printf("Finshed checking client heartbeats.")
		s.clientsLock.Unlock()
	}
}
