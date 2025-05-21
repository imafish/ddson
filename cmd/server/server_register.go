package main

import (
	"fmt"
	"log"
	"time"

	"internal/pb"
	"internal/version"
)

func (s *server) Register(req *pb.RegisterRequest, stream pb.DDSONService_RegisterServer) error {

	// check if version is compatible
	clientVersion, err := version.VersionFromString(req.Version)
	if err != nil {
		return fmt.Errorf("invalid version format: %s", req.Version)
	}
	if !version.VersionCompatible(version.CurrentVersion(), clientVersion) {
		return fmt.Errorf("version mismatch: server %s, client %s", version.CurrentVersion(), clientVersion)
	}
	// Check if client already exists
	if _, exists := s.clients.getClientByName(req.Name); exists {
		return fmt.Errorf("client %s already registered", req.Name)
	}

	// Create new client
	s.clients.addClient(req.Name, req.Version, stream)
	client, _ := s.clients.getClientByName(req.Name)

	log.Printf("Client registered: %s (version: %s)", req.Name, req.Version)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Keep the connection open
	for {
		select {
		case <-stream.Context().Done():
			log.Printf("Client %s disconnected, removing from client list", req.Name)
			s.clients.removeClient(req.Name)
			return nil

		case <-client.done:
			log.Printf("client %s is marked closed, exit loop", req.Name)
			return nil

		case <-ticker.C:
			log.Printf("Sending message to client %s", req.Name)
			connectedClients := make([]string, 0)
			s.clients.mtx.Lock()
			for name := range s.clients.clients {
				connectedClients = append(connectedClients, name)
			}
			s.clients.mtx.Unlock()
			msg := &pb.ServerMessage{
				Type:      pb.ServerMessageType_MESSAGE,
				Message:   fmt.Sprintf("Connected clients: %v", connectedClients),
				Timestamp: time.Now().Unix(),
			}
			if err := stream.Send(msg); err != nil {
				log.Printf("Failed to send message to %s: %v", req.Name, err)
				s.clients.removeClient(req.Name)
				return err
			}
		}
	}
}
