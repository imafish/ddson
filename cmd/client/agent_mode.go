package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/bgentry/go-netrc/netrc" // Third-party library
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"internal/pb"
	"internal/version"
)

var (
	quit bool = false
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

func run_agent() {
	// start grpc server and heartbeat thread

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
	for !quit {
		agent(id)

		log.Printf("Agent stopped, restarting in 5 seconds...")
		time.Sleep(5 * time.Second)
	}
	log.Printf("quit message received, stopping heartbeat")
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

	go sendHeartbeats(client, *id)
}

func (c *client) DownloadPart(grpcRequest *pb.DownloadPartRequest, stream pb.DDSONServiceClient_DownloadPartServer) error {
	url, offset, size, clientId, taskId, subtaskId := grpcRequest.Url, grpcRequest.Offset, grpcRequest.Size, grpcRequest.ClientId, grpcRequest.TaskId, grpcRequest.SubtaskId
	log.Printf("Received download request: URL=%s, Offset=%d, Size=%d, ClientId=%d, TaskId=%d, SubtaskId=%d", url, offset, size, clientId, taskId, subtaskId)

	// Parse .netrc file for credentials
	username, password, err := getDataFromNetrc(url)
	if err != nil {
		log.Printf("Failed to parse .netrc file: %v", err)
		return err
	}

	// Create HTTP request with Range header
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Failed to create HTTP request: %v", err)
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+size-1))
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	// Perform the HTTP request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Failed to download file: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		log.Printf("Unexpected HTTP status: %s", resp.Status)
		return fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	// Read the response body into a buffer
	buffer := make([]byte, size)
	n, err := io.ReadFull(resp.Body, buffer)
	if err != nil && err != io.EOF {
		log.Printf("Failed to read response body: %v", err)
		return err
	}
	log.Printf("Downloaded %d bytes from URL: %s", n, url)
	if int64(n) < size {
		log.Printf("Read less data than expected: %d bytes", n)
		return fmt.Errorf("read less data than expected: downloaded: %d, expected: %d", n, size)
	}

	i := 0
	j := 0
	for n > 0 {
		// Send the data in chunks
		chunkSize := 1024 * 1024 // 1 MB chunk size
		if n < chunkSize {
			chunkSize = n
		}
		j = i + chunkSize

		log.Printf("Uploading [%d-%d) bytes to server...", i, j)
		err = stream.Send(&pb.DownloadStatus{
			Status: pb.DownloadStatusType_TRANSFERRING,
			Data:   buffer[i:j],
		})
		if err != nil {
			log.Printf("Failed to send upload data: %v", err)
			return err
		}

		n -= chunkSize
		i += chunkSize
	}
	log.Printf("Upload completed.")
	return nil
}

func sendHeartbeats(client pb.DDSONServiceClient, id int32) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	errCount := 0

	for !quit {
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
			errCount--
			log.Printf("Heartbeat successful: count: %d, message from server: %s", errCount, resp.Message)
		} else {
			// resp.Success is false
			log.Printf("Heartbeat rejected: count: %d, message from server: %s", errCount, resp.Message)
			errCount++
		}

		if errCount > 3 {
			log.Printf("Too many errors, disconnecting...")
			quit = true
			return
		}
	}
}

func getDataFromNetrc(downloadUrl string) (string, string, error) {
	// Parse URL
	parsedURL, err := url.Parse(downloadUrl)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}

	// Parse .netrc
	netrcPath := os.ExpandEnv("$HOME/.netrc")
	if stat, err := os.Stat(netrcPath); err == nil && !stat.IsDir() {
		nrc, err := netrc.ParseFile(netrcPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to parse .netrc: %w", err)
		}

		// Find machine entry
		machine := nrc.FindMachine(parsedURL.Host)
		if machine == nil {
			return "", "", fmt.Errorf("no credentials found for host: %s", parsedURL.Host)
		}
		return machine.Login, machine.Password, nil
	}
	return "", "", fmt.Errorf("no .netrc file found")
}
