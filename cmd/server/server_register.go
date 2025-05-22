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
	client := s.clients.addClient(req.Name, req.Version, stream, s.freeId)
	s.freeId++

	log.Printf("Client registered: %s (version: %s)", req.Name, req.Version)
	err = stream.Send(&pb.ServerMessage{
		Type:      pb.ServerMessageType_REGISTER_OK,
		Timestamp: time.Now().Unix(),
		Message:   fmt.Sprintf("Registration successful, client ID: %d", client.id),
		Id:        client.id,
	})
	if err != nil {
		log.Printf("Failed to send registration response: %v", err)
		return err
	}

	return client.handleRegistration(req, stream, s)
}
