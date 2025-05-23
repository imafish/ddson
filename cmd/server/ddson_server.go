package main

import (
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"internal/pb"
)

type server struct {
	pb.UnimplementedDDSONServiceServer
	clients  *clientList
	taskList *taskList
}

func newServer() *server {
	return &server{
		clients:  newClientList(),
		taskList: newTaskList(),
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
		grpc.MaxRecvMsgSize(100*1024*1024),
		grpc.MaxSendMsgSize(100*1024*1024), // 100 MB
	)

	serverInstance := newServer()
	pb.RegisterDDSONServiceServer(s, serverInstance)

	// Start client monitoring goroutine
	go serverInstance.monitorClients()

	// Start task processing goroutine
	go serverInstance.runTasks()

	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func (s *server) runTasks() {
	s.taskList.run(s)
}
