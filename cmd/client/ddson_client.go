package main

import (
	"context"
	"flag"
	"log"
	"net/url"
	"path"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"internal/pb"
)

const (
	version = "1.0.0"
)

var (
	addr        = flag.String("addr", "localhost:5510", "the address to connect to")
	clientName  = flag.String("name", "", "the name of the client")
	downloadUrl = flag.String("url", "", "URL to download from")
	output      = flag.String("output", "", "output file name")
)

func main() {
	flag.Parse()

	if *clientName == "" && *downloadUrl == "" {
		log.Fatal("at least one of --name or --url must be specified")
	}

	if *downloadUrl != "" {
		// downloader mode
		if *output == "" {
			parsedURL, err := url.Parse(*downloadUrl)
			if err != nil {
				log.Fatalf("failed to parse URL: %v", err)
			}

			pathSegments := parsedURL.Path
			*output = path.Base(pathSegments)
			log.Printf("Extracted file name from URL: %s", *output)
		}

		log.Printf("Downloading from %s to %s", *downloadUrl, *output)
		download()

	} else {

		// client daemon mode
		log.Printf("starting client %s (version: %s)", *clientName, version)
		log.Printf("server is: %s", *addr)

		for {
			connect()
			time.Sleep(5 * time.Second)
		}
	}
}

func download() {
	// Implement the download logic here
	log.Printf("Downloading from %s to %s", *downloadUrl, *output)
}

func connect() {
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

	quit := false
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

		state := pb.ClientState_IDLE

		resp, err := client.Heartbeat(context.Background(), &pb.HeartbeatRequest{
			Name:  *clientName,
			State: state,
		})
		if err != nil {
			log.Printf("Heartbeat failed: %v", err)
			errCount++
		} else if resp.Success {
			log.Printf("Heartbeat sent (state: %v)", state)
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
