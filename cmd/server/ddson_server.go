package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"internal/logging"
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

var logger *slog.Logger

func main() {
	debug := flag.Bool("debug", false, "enable debug mode (default: false)")
	port := flag.Int("port", 5510, "the port to listen on (default: 5510)")
	flag.Parse()

	// Set up slog logger
	var logger *slog.Logger
	loglevel := slog.LevelInfo
	if *debug {
		loglevel = slog.LevelDebug
	}
	// if stdout is a terminal, use colorized output, otherwise use plain text
	useColor := term.IsTerminal(int(os.Stdout.Fd()))
	logger = logging.NewCustomLogger(os.Stdout, loglevel, useColor)
	slog.SetDefault(logger)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		slog.Error("failed to listen", "error", err)
		os.Exit(1)
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

	slog.Info("Server listening", "address", lis.Addr())
	if err := s.Serve(lis); err != nil {
		slog.Error("failed to serve", "error", err)
		os.Exit(1)
	}
}

func (s *server) runTasks() {
	s.taskList.run(s)
}
