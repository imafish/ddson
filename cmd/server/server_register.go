package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"internal/pb"
	"internal/version"

	"google.golang.org/grpc/peer"
)

func (s *server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	slog.Debug("Client registering", "name", req.Name, "version", req.Version)

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

	err = s.clients.clientAllowed(req.Name)
	if err != nil {
		return nil, err
	}

	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("failed to get peer information")
	}
	if p.Addr == nil {
		return nil, fmt.Errorf("failed to get client address")
	}
	clientAddr := p.Addr.String()
	// get client address without port
	clientAddr = clientAddr[:len(clientAddr)-len(fmt.Sprintf(":%d", p.Addr.(*net.TCPAddr).Port))]
	clientPort := int(req.Port)
	slog.Debug("Client info", "address", clientAddr, "port", clientPort, "version", req.Version, "name", req.Name)

	// Create new client
	freeId := s.clients.addClient(req.Name, req.Version, clientAddr, clientPort)

	slog.Info("Client registered", "name", req.Name, "id", freeId, "address", clientAddr, "port", clientPort)

	return &pb.RegisterResponse{
		Id:            int32(freeId),
		ServerVersion: version.CurrentVersion().String(),
	}, nil
}
