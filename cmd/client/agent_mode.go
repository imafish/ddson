package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	// Third-party library
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"internal/pb"
	"internal/version"
)

type client struct {
	pb.UnimplementedDDSONServiceClientServer
	id    int32
	state pb.ClientState
}

func newClient() *client {
	return &client{
		id:    0,
		state: pb.ClientState_IDLE,
	}
}

func runAgent() {
	// start grpc server and heartbeat thread
	listenAddr := fmt.Sprintf(":%d", *servicePort)

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		slog.Error("Failed to listen", "error", err)
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
	client := newClient()
	pb.RegisterDDSONServiceClientServer(s, client)

	// heartbeat thread
	go sendHeartBeatsToServer(&client.id)

	slog.Info("Client agent listening", "address", lis.Addr())
	if err := s.Serve(lis); err != nil {
		slog.Error("Failed to serve", "error", err)
		os.Exit(1)
	}
}

func sendHeartBeatsToServer(id *int32) {
	for {
		agent(id)

		slog.Warn("Agent stopped, restarting in 5 seconds...")
		time.Sleep(5 * time.Second)
	}
}

func agent(id *int32) {
	slog.Info("Connecting to server", "address", *addr)
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("Failed to connect", "error", err)
		return
	}
	defer conn.Close()

	client := pb.NewDDSONServiceClient(conn)

	// Start registration stream
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	response, err := client.Register(ctx, &pb.RegisterRequest{
		Name:    *clientName,
		Version: version.VersionString,
		Port:    int32(*servicePort),
	})
	if err != nil {
		slog.Error("Register failed", "error", err)
		return
	}

	*id = response.Id
	slog.Info("Registered successfully", "id", response.Id, "serverVersion", response.ServerVersion)

	sendHeartbeats(client, *id)
}

func sendHeartbeats(client pb.DDSONServiceClient, id int32) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	errCount := 0

	for {
		<-ticker.C

		slog.Log(context.Background(), slog.LevelDebug-1, "Sending heartbeat to server...")
		resp, err := client.Heartbeat(context.Background(), &pb.HeartbeatRequest{
			Name: *clientName,
			Id:   id,
		})
		if err != nil {
			errCount++
			slog.Warn("Failed to send heartbeat", "count", errCount, "error", err)
		} else if resp.Success {
			if errCount > 0 {
				errCount--
			}
			slog.Log(context.Background(), slog.LevelDebug-1, "Heartbeat successful", "count", errCount, "message", resp.Message)
		} else {
			// resp.Success is false
			slog.Warn("Heartbeat rejected", "count", errCount, "message", resp.Message)
			errCount++
		}

		if errCount > 3 {
			slog.Error("Too many errors, disconnecting...")
			return
		}
	}
}
