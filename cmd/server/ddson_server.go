package main

import (
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"internal/pb"
)

type server struct {
	pb.UnimplementedDDSONServiceServer
	clients     map[string]*clientInfo
	clientsLock sync.RWMutex
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
		grpc.MaxRecvMsgSize(100*1024*1024),
		grpc.MaxSendMsgSize(100*1024*1024), // 100 MB
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
