package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"internal/pb"
)

type clientInfo struct {
	name      string
	version   string
	state     pb.ClientState
	lastSeen  time.Time
	stream    pb.DDSONService_RegisterServer
	connected bool
}

type server struct {
	pb.UnimplementedDDSONServiceServer
	clients     map[string]*clientInfo
	clientsLock sync.RWMutex
}

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
			msg := fmt.Sprintf("Connected clients: %v", connectedClients)
			if client.connected {
				msg := &pb.ServerMessage{
					Type:      pb.ServerMessageType_MESSAGE,
					Message:   msg,
					Timestamp: time.Now().Unix(),
				}
				if err := stream.Send(msg); err != nil {
					log.Printf("Failed to send message to %s: %v", req.Name, err)
					client.connected = false
					return err
				}
			}
		}
	}
}

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
				client.connected = false
			}
		}
		log.Printf("Finshed checking client heartbeats.")
		s.clientsLock.Unlock()
	}
}

func main() {
	lis, err := net.Listen("tcp", ":5510")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    10 * time.Second,
			Timeout: 20 * time.Second,
		}),
	)

	serverInstance := &server{
		clients: make(map[string]*clientInfo),
	}
	pb.RegisterDDSONServiceServer(s, serverInstance)

	// Start client monitoring goroutine
	go serverInstance.monitorClients()

	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
