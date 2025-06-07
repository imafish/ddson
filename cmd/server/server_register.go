package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"internal/agents"
	"internal/pb"
	"internal/version"

	"google.golang.org/grpc/peer"
)

func (s *server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	slog.Debug("Agent registering", "name", req.Name, "version", req.Version)

	// check if version is compatible
	agentVersion, err := version.VersionFromString(req.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid version format: %s", req.Version)
	}
	if !version.VersionCompatible(version.CurrentVersion(), agentVersion) {
		return nil, fmt.Errorf("version mismatch: server %s, agent %s", version.CurrentVersion(), agentVersion)
	}

	// get agent IP
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("failed to get peer information")
	}
	if p.Addr == nil {
		return nil, fmt.Errorf("failed to get agent address")
	}
	agentAddr := p.Addr.String()
	// get agent address without port
	agentAddr = agentAddr[:len(agentAddr)-len(fmt.Sprintf(":%d", p.Addr.(*net.TCPAddr).Port))]
	port := int(req.Port)

	addr := net.JoinHostPort(agentAddr, fmt.Sprintf("%d", port))
	slog.Debug("Agent info", "address", addr, "port", port, "version", req.Version, "name", req.Name)

	// Create new agent
	newAgent := agents.NewAgent(req.Name, req.Version, addr)
	id, err := s.agentList.AddAgent(newAgent)
	if err != nil {
		slog.Error("Failed to register agent", "error", err, "name", req.Name, "address", agentAddr, "port", port)
		return nil, err
	}
	heartbeatTimer := time.NewTimer(20 * time.Second)
	s.heartbeatTimers[id] = heartbeatTimer

	go func() {
		<-heartbeatTimer.C
		slog.Debug("Heartbeat timer expired, removing agent", "agentID", id, "name", req.Name, "address", addr)
		newAgent.Retire()
		s.agentList.RemoveAgent(id)
		delete(s.heartbeatTimers, id)
	}()

	slog.Info("Agent registered", "name", req.Name, "id", id, "address", addr)
	return &pb.RegisterResponse{
		Id:            int32(id),
		ServerVersion: version.CurrentVersion().String(),
	}, nil
}
