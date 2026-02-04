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

	"github.com/imafish/ddson/internal/agents"
	"github.com/imafish/ddson/internal/common"
	"github.com/imafish/ddson/internal/downloadtask"
	"github.com/imafish/ddson/internal/logging"
	"github.com/imafish/ddson/internal/pb"
	"github.com/imafish/ddson/internal/persistency"
)

type server struct {
	pb.UnimplementedDDSONServiceServer
	agentManager *agents.AgentManager
	taskManager  downloadtask.TaskManager
	persistency  *persistency.Persistency
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

	agentManager := agents.NewAgentManagerWithDefaultConfig()
	taskManager := downloadtask.NewTaskManagerImpl()
	return &server{
		agentManager: agentManager,
		taskManager:  taskManager,
		persistency:  p,
	}
}

func main() {
	debug := flag.Bool("debug", false, "enable debug mode (default: false)")
	port := flag.Int("port", 5510, "the port to listen on (default: 5510)")
	verbose := flag.Bool("verbose", false, "enable verbose logging (default: false)")
	flag.Parse()

	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	if *verbose {
		logLevel = slog.LevelDebug - 1
	}

	// if stdout is a terminal, use colorized output, otherwise use plain text
	useColor := term.IsTerminal(int(os.Stdout.Fd()))
	logger := logging.NewCustomLogger(logLevel, useColor, "")
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

	go serverInstance.runTasks()

	slog.Info("Server listening", "address", lis.Addr())
	if err := s.Serve(lis); err != nil {
		slog.Error("failed to serve", "error", err)
		os.Exit(1)
	}
}

func (s *server) runTasks() {
	s.taskManager.Run(func(task *downloadtask.Task) error {
		return task.Run()
	})
}
