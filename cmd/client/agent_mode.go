package main

import (
	"context"
	"fmt"
	"log"
	"net"
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
	client := newClient()
	pb.RegisterDDSONServiceClientServer(s, client)

	// heartbeat thread
	go sendHeartBeatsToServer(&client.id)

	log.Printf("Client agent listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func sendHeartBeatsToServer(id *int32) {
	for {
		agent(id)

		log.Printf("Agent stopped, restarting in 5 seconds...")
		time.Sleep(5 * time.Second)
	}
}

func agent(id *int32) {
	log.Printf("Connecting to %s...", *addr)
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("did not connect: %v", err)
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
		log.Printf("Register failed: %v", err)
		return
	}

	*id = response.Id
	log.Printf("Registered, id: %d, server version: %s", response.Id, response.ServerVersion)

	sendHeartbeats(client, *id)
}

func sendHeartbeats(client pb.DDSONServiceClient, id int32) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	errCount := 0

	for {
		<-ticker.C

		log.Printf("Sending heartbeat to server...")
		resp, err := client.Heartbeat(context.Background(), &pb.HeartbeatRequest{
			Name: *clientName,
			Id:   id,
		})
		if err != nil {
			errCount++
			log.Printf("Failed to send heartbeat, count: %d, error: %v", errCount, err)
		} else if resp.Success {
			if errCount > 0 {
				errCount--
			}
			log.Printf("Heartbeat successful: count: %d, message from server: %s", errCount, resp.Message)
		} else {
			// resp.Success is false
			log.Printf("Heartbeat rejected: count: %d, message from server: %s", errCount, resp.Message)
			errCount++
		}

		if errCount > 3 {
			log.Printf("Too many errors, disconnecting...")
			return
		}
	}
}
