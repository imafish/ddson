package main

import (
	"errors"
	"fmt"
	"log"
	"time"

	"internal/pb"
)

func (s *server) Register(req *pb.RegisterRequest, stream pb.DDSONService_RegisterServer) error {
	s.clientsLock.Lock()

	// Check if client already exists
	if _, exists := s.clients[req.Name]; exists {
		s.clientsLock.Unlock()
		return errors.New("client already registered")
	}

	// Create new client
	client := &clientInfo{
		name:      req.Name,
		version:   req.Version,
		state:     pb.ClientState_IDLE,
		lastSeen:  time.Now(),
		stream:    stream,
		connected: true,
	}
	s.clients[req.Name] = client
	s.clientsLock.Unlock()

	log.Printf("Client registered: %s (version: %s)", req.Name, req.Version)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Keep the connection open
	for {
		select {
		case <-stream.Context().Done():
			log.Printf("Client %s disconnected", req.Name)
			s.clientsLock.Lock()
			delete(s.clients, req.Name)
			s.clientsLock.Unlock()
			return nil

		case <-ticker.C:
			log.Printf("Sending message to client %s", req.Name)
			// Send periodic messages to client
			connectedClients := make([]string, 0)
			s.clientsLock.RLock()
			for name := range s.clients {
				connectedClients = append(connectedClients, name)
			}
			s.clientsLock.RUnlock()
			msg := &pb.ServerMessage{
				Type:      pb.ServerMessageType_MESSAGE,
				Message:   fmt.Sprintf("Connected clients: %v", connectedClients),
				Timestamp: time.Now().Unix(),
			}
			if err := stream.Send(msg); err != nil {
				log.Printf("Failed to send message to %s: %v", req.Name, err)
				client.close()
				return err
			}
		}
	}
}
