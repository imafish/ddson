package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/bgentry/go-netrc/netrc" // Third-party library
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"internal/pb"
	"internal/version"
)

var clientState pb.ClientState = pb.ClientState_IDLE

func run_agent() {
	for {
		quitChan := make(chan bool, 2)
		agent(quitChan)

		log.Printf("Agent stopped, restarting in 5 seconds...")
		time.Sleep(5 * time.Second)
	}
}

func agent(quitChan chan bool) {
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

	defer func() {
		quitChan <- true
	}()

	registerStream, err := client.Register(ctx, &pb.RegisterRequest{
		Name:    *clientName,
		Version: version.VersionString,
	})
	if err != nil {
		log.Printf("Register failed: %v", err)
		return
	}

	log.Printf("Registered as %s (version: %s)", *clientName, version.VersionString)

	go sendHeartbeats(quitChan, client)

	handleIncomingMessages(registerStream, quitChan, client)
}

func sendHeartbeats(quitChan chan bool, client pb.DDSONServiceClient) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	errCount := 0

	for {
		select {
		case <-quitChan:
			log.Printf("quit message received, stopping heartbeat")
			return

		case <-ticker.C:
			log.Printf("Sending heartbeat to server...")

			resp, err := client.Heartbeat(context.Background(), &pb.HeartbeatRequest{
				Name:  *clientName,
				State: clientState,
				Id:    myId,
			})
			if err != nil {
				errCount++
				log.Printf("Failed to send heartbeat, count: %d, error: %v", errCount, err)
			} else if resp.Success {
				log.Printf("Heartbeat sent (state: %v)", clientState)
				errCount--
			} else {
				// resp.Success is false
				log.Printf("Heartbeat rejected: count: %d, error: %s", errCount, resp.Message)
				errCount++
			}

			if errCount > 3 {
				log.Printf("Too many errors, disconnecting...")
				quitChan <- true
				return
			}
		}
	}
}

func handleIncomingMessages(stream pb.DDSONService_RegisterClient, quitChan chan bool, client pb.DDSONServiceClient) {
	defer func() {
		quitChan <- true
	}()

	for {
		select {
		case <-quitChan:
			log.Printf("quit message received, stopping message handling")
			return
		case <-stream.Context().Done():
			log.Printf("Stream context done, stopping message handling")
			return
		default:
			// Continue processing messages
		}

		msg, err := stream.Recv()
		if err != nil {
			log.Printf("Failed to receive message: %v", err)
			return
		}

		log.Printf("Received from server: %s (timestamp: %d)", msg.Message, msg.Timestamp)
		switch msg.Type {
		case pb.ServerMessageType_MESSAGE:
			log.Printf("Received message from server: %s (timestamp: %d)", msg.Message, msg.Timestamp)

		case pb.ServerMessageType_ERROR:
			log.Printf("Error from server: %s", msg.Message)
			return

		case pb.ServerMessageType_REGISTER_OK:
			log.Printf("Registration successful, client ID: %d", msg.Id)
			myId = msg.Id

		case pb.ServerMessageType_TASK:
			log.Printf("Task from server: %s (timestamp: %d)", msg.Message, msg.Timestamp)
			if clientState != pb.ClientState_IDLE {
				log.Printf("Client is busy, ignoring task")
				// TODO: send error message to server
			} else {
				clientState = pb.ClientState_BUSY
				err = runTask(client, msg)
				clientState = pb.ClientState_IDLE
				if err != nil {
					// TODO: send error message to server
					log.Printf("Failed to run task: %v", err)
				}
			}
		}
	}
}

func runTask(client pb.DDSONServiceClient, msg *pb.ServerMessage) error {
	url, offset, size, id := msg.Url, msg.Offset, msg.Size, msg.Id
	log.Printf("Running task #%d: URL: %s (offset: %d, size: %d)", id, url, offset, size)

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

	// Upload the content to the server using the Upload RPC
	stream, err := client.Upload(context.Background())
	if err != nil {
		log.Printf("Failed to start upload stream: %v", err)
		return err
	}

	log.Printf("Uploading %d bytes to server...", n)
	for n > 0 {
		// Send the data in chunks
		chunkSize := int64(1024 * 1024) // 1 MB chunk size
		if n < int(chunkSize) {
			chunkSize = int64(n)
		}

		err = stream.Send(&pb.UploadData{
			Url:    url,
			Id:     id,
			Offset: offset,
			Size:   chunkSize,
			Data:   buffer[:chunkSize],
		})
		if err != nil {
			log.Printf("Failed to send upload data: %v", err)
			return err
		}

		offset += chunkSize
		n -= int(chunkSize)
	}
	log.Printf("Sent completed.")

	// Close the stream and receive the server's response
	respStatus, err := stream.CloseAndRecv()
	if err != nil {
		log.Printf("Failed to complete upload: %v", err)
		return err
	}

	if !respStatus.Success {
		log.Printf("Upload failed: %s", respStatus.Message)
		// TODO: retry, ignore?
		return nil
	}

	log.Printf("Upload completed successfully")
	return nil
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
