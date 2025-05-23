package main

import (
	"context"
	"fmt"
	"log"

	"internal/pb"
	"internal/version"

	"google.golang.org/grpc/peer"
)

func (s *server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {

	// check if version is compatible
	clientVersion, err := version.VersionFromString(req.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid version format: %s", req.Version)
	}
	if !version.VersionCompatible(version.CurrentVersion(), clientVersion) {
		return nil, fmt.Errorf("version mismatch: server %s, client %s", version.CurrentVersion(), clientVersion)
	}
	// Check if client already exists
	if _, exists := s.clients.getClientByName(req.Name); exists {
		return nil, fmt.Errorf("client %s already registered", req.Name)
	}

	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("failed to get peer information")
	}
	if p.Addr == nil {
		return nil, fmt.Errorf("failed to get client address")
	}
	clientAddr := p.Addr.String()
	clientPort := int(req.Port)

	log.Printf("Client %s (%s:%d) is registering with version %s", req.Name, clientAddr, clientPort, req.Version)

	// Create new client
	freeId := s.clients.addClient(req.Name, req.Version, clientAddr, clientPort)

	return &pb.RegisterResponse{
		Id:            int32(freeId),
		ServerVersion: version.CurrentVersion().String(),
	}, nil
}
