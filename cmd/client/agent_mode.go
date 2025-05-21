package main

import (
	"bytes"
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
)

var clientState pb.ClientState = pb.ClientState_IDLE
var quit bool = false

func run_agent() {
	for {
		agent()
		time.Sleep(5 * time.Second)
	}
}

func agent() {
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
		quit = true
	}()

	registerStream, err := client.Register(ctx, &pb.RegisterRequest{
		Name:    *clientName,
		Version: version,
	})
	if err != nil {
		log.Printf("Register failed: %v", err)
		return
	}

	log.Printf("Registered as %s (version: %s)", *clientName, version)

	// Goroutine to receive messages from server
	go func() {
		defer func() {
			quit = true
		}()

		for !quit {
			msg, err := registerStream.Recv()
			if err != nil {
				log.Printf("Failed to receive message: %v", err)
				return
			}

			switch msg.Type {
			case pb.ServerMessageType_MESSAGE:
				log.Printf("Received message from server: %s (timestamp: %d)", msg.Message, msg.Timestamp)
			case pb.ServerMessageType_ERROR:
				log.Printf("Error from server: %s", msg.Message)
				quit = true
				return
			case pb.ServerMessageType_TASK:
				log.Printf("Task from server: %s (timestamp: %d)", msg.Message, msg.Timestamp)
				if clientState != pb.ClientState_IDLE {
					log.Printf("Client is busy, ignoring task")
					// TODO: send error message to server
				} else {
					clientState = pb.ClientState_BUSY
					err = runTask(client, msg.GetMessage(), msg.GetOffset(), msg.GetSize())
					if err != nil {
						log.Printf("Failed to run task: %v", err)
					}
					clientState = pb.ClientState_IDLE
				}
			}

			log.Printf("Received from server: %s (timestamp: %d)", msg.Message, msg.Timestamp)
		}
		log.Printf("Stopped receiving messages from server")
	}()

	// Send periodic heartbeats
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	errCount := 0

	for range ticker.C {
		if quit {
			log.Printf("Stopping heartbeat")
			break
		}

		log.Printf("Sending heartbeat to server...")

		resp, err := client.Heartbeat(context.Background(), &pb.HeartbeatRequest{
			Name:  *clientName,
			State: clientState,
		})
		if err != nil {
			log.Printf("Heartbeat failed: %v", err)
			errCount++
		} else if resp.Success {
			log.Printf("Heartbeat sent (state: %v)", clientState)
			errCount--
		} else {
			// resp.Success is false
			log.Printf("Heartbeat rejected: %s", resp.Message)
			errCount++
		}

		if errCount > 3 {
			log.Printf("Too many errors, disconnecting...")
			break
		}
	}
}

func runTask(client pb.DDSONServiceClient, url string, offset int64, size int64) error {
	log.Printf("Running task: URL: %s (offset: %d, size: %d)", url, offset, size)

	// Parse .netrc file for credentials
	username, password, err := GetDataFromNetrc(url)
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
		// TODO: retry?
		return fmt.Errorf("upload failed: %s", respStatus.Message)
	}

	log.Printf("Upload completed successfully")
	return nil
}

func GetDataFromNetrc(downloadUrl string) (string, string, error) {
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

func DownloadWithNetrcLibrary(fileURL string) (*bytes.Buffer, error) {

	login, password, err := GetDataFromNetrc(fileURL)
	if err != nil {
		log.Printf("Failed to get credentials from .netrc: %v", err)
	}

	// Create HTTP httpClient
	httpClient := &http.Client{}

	// Create request with basic auth
	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if login != "" && password != "" {
		req.SetBasicAuth(login, password)
	}

	// Execute request
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	// Create a byte buffer to store the response body
	buffer := &bytes.Buffer{}

	// Write the body to file
	_, err = io.Copy(buffer, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	log.Printf("Downloaded %d bytes from %s", buffer.Len(), fileURL)
	return buffer, nil
}
