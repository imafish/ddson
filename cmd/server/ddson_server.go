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

	"internal/agents"
	"internal/common"
	"internal/logging"
	"internal/pb"
	"internal/persistency"
)

type server struct {
	pb.UnimplementedDDSONServiceServer
	agentList       agents.AgentList
	taskList        *taskList
	heartbeatTimers map[int]*time.Timer
	persistency     *persistency.Persistency
}

func newServer() *server {
	homeDir, err := common.OriginalUserHomeDir()
	if err != nil {
		slog.Error("failed to get original user home directory", "error", err)
		os.Exit(1)
	}
	workspaceDir := fmt.Sprintf("%s/workspace_ddson", homeDir)
	p, err := persistency.NewAndInitializePersistency(workspaceDir)
	if err != nil {
		slog.Error("failed to create persistency", "error", err)
		os.Exit(1)
	}

	return &server{
		agentList:       agents.NewAgentList(),
		taskList:        newTaskList(),
		heartbeatTimers: make(map[int]*time.Timer),
		persistency:     p,
	}
}

func main() {
	debug := flag.Bool("debug", false, "enable debug mode (default: false)")
	port := flag.Int("port", 5510, "the port to listen on (default: 5510)")
	verbose := flag.Bool("verbose", false, "enable verbose logging (default: false)")
	flag.Parse()

	// Set up slog logger
	var logger *slog.Logger
	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	if *verbose {
		logLevel = slog.LevelDebug - 1
	}

	// if stdout is a terminal, use colorized output, otherwise use plain text
	useColor := term.IsTerminal(int(os.Stdout.Fd()))
	logger = logging.NewCustomLogger(os.Stdout, logLevel, useColor)
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
