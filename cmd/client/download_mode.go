package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"internal/pb"
	"internal/version"
)

func download() {
	// Establish a connection to the server
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to server: %v", err)
	}
	defer conn.Close()

	client := pb.NewDDSONServiceClient(conn)

	// Create a DownloadRequest
	req := &pb.DownloadRequest{
		Version:  version.VersionString,
		Url:      *downloadUrl,
		Checksum: "",
	}

	// Open the output file
	file, err := os.Create(*output)
	if err != nil {
		log.Fatalf("failed to create output file: %v", err)
	}
	defer file.Close()

	// Send the request and receive the stream
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.Download(ctx, req)
	if err != nil {
		log.Fatalf("failed to start download: %v", err)
	}

	var received int64

	// Process the responses from the server
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				log.Println("Download completed")
				break
			}
			log.Fatalf("error receiving data: %v", err)
		}

		switch resp.GetStatus() {
		case pb.DownloadStatusType_TRANSFERRING:
			log.Printf("Start receiving data from server...")
			// Write data to the file
			if _, err := file.Write(resp.GetData()); err != nil {
				log.Fatalf("failed to write data to file: %v", err)
			}
			received += int64(len(resp.GetData()))
			log.Printf("Received %s", prettyFormatSize(received))

		case pb.DownloadStatusType_PENDING:
			log.Printf("Download is pending, number #%d in queue", resp.GetProgress())
		case pb.DownloadStatusType_DOWNLOADING:
			log.Print(fmtProgress(resp))
		case pb.DownloadStatusType_VALIDATING:
			log.Printf("Validating integrity...")
		default:
			// Print the status message
			fmt.Printf("Status: %s, Message: %s\n", resp.GetStatus().String(), resp.GetMessage())
		}
	}
}

func fmtProgress(resp *pb.DownloadStatus) string {
	downloadSize := prettyFormatSize(resp.GetDownloaded())
	totalSize := prettyFormatSize(resp.GetTotal())
	speed := prettyFormatSpeed(resp.GetSpeed())
	eta := prettyFormatDuration(resp.GetTotal()-resp.GetDownloaded(), resp.GetSpeed())
	percentage :=
		fmt.Sprintf("%.2f%%", float64(resp.GetDownloaded())/float64(resp.GetTotal())*100)
	return fmt.Sprintf("Downloading... [%s] %s/%s, speed: %s, ETA: %s", percentage, downloadSize, totalSize, speed, eta)
}

func prettyFormatSize(size int64) string {
	switch {
	case size >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(size)/(1<<30))
	case size >= 1<<20:
		return fmt.Sprintf("%.2f MB", float64(size)/(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf("%.2f KB", float64(size)/(1<<10))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func prettyFormatSpeed(speed int32) string {
	// Convert speed to a human-readable format
	switch {
	case speed >= 1<<30:
		return fmt.Sprintf("%.2f GB/s", float64(speed)/(1<<30))
	case speed >= 1<<20:
		return fmt.Sprintf("%.2f MB/s", float64(speed)/(1<<20))
	case speed >= 1<<10:
		return fmt.Sprintf("%.2f KB/s", float64(speed)/(1<<10))
	default:
		return fmt.Sprintf("%d B/s", speed)
	}
}

func prettyFormatDuration(duration int64, speed int32) string {
	// Convert duration to a human-readable format
	if speed == 0 {
		return "N/A"
	}
	seconds := duration / int64(speed)
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	seconds = seconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
