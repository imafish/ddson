package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"internal/common"
	"internal/httputil"
	"internal/pb"
)

func download() {
	supportPartialDownload, totalSize, err := httputil.CheckPartialDownloadSupport(*downloadUrl)
	if err != nil {
		slog.Error("Failed to check partial download support", "error", err)
		os.Exit(1)
	}
	if !supportPartialDownload {
		slog.Error("Server does not support partial downloads, please use other tools")
		os.Exit(1)
	}

	// Establish a connection to the server
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("Failed to connect to server", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := pb.NewDDSONServiceClient(conn)

	// Create a DownloadRequest
	req := &pb.DownloadRequest{
		ClientId: int32(0), // TODO: currently client id is ignored. later will be used to identify the client
		Url:      *downloadUrl,
		Checksum: *sha256,
	}

	// Send the request and receive the stream
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.Download(ctx, req)
	if err != nil {
		slog.Error("Failed to start download", "error", err)
		os.Exit(1)
	}

	// Open the output file
	var file *os.File = nil
	var received int64 = 0
	transferringStarted := false

	// Process the responses from the server
	for {
		resp, err := stream.Recv()

		// Create file after receiving the first response
		if file == nil && (err == nil || err == io.EOF) {
			file, err = os.Create(*output)
			if err != nil {
				slog.Error("Failed to create output file", "error", err)
				os.Exit(1)
			}
			defer file.Close()
		}

		if err != nil {
			if err == io.EOF {
				slog.Info("Download completed")
				break
			}
			slog.Error("Error receiving data", "error", err)
			os.Exit(1)
		}

		switch resp.GetStatus() {
		case pb.DownloadStatusType_DOWNLOADING:
			slog.Info(fmtProgress(resp, totalSize))

		case pb.DownloadStatusType_TRANSFERRING:
			if !transferringStarted {
				slog.Info("Start receiving data from server...")
				transferringStarted = true
			}
			// Write data to the file
			if _, err := file.Write(resp.GetData()); err != nil {
				slog.Error("Failed to write data to file", "error", err)
				os.Exit(1)
			}
			received += int64(len(resp.GetData()))
			slog.Debug("Received data", "size", common.PrettyFormatSize(received))

		case pb.DownloadStatusType_PENDING:
			slog.Warn("Download is in queue", "queuePosition", resp.NumberInQueue, "clientCount", resp.ClientCount)

		case pb.DownloadStatusType_VALIDATING:
			slog.Info("Validating integrity...")
		}
	}
}

func fmtProgress(resp *pb.DownloadStatus, totalSize int64) string {
	downloadedBytes := resp.GetTotalDownloadedBytes()
	downloadedBytesStr := common.PrettyFormatSize(downloadedBytes)
	totalSizeStr := common.PrettyFormatSize(totalSize)
	speed := common.PrettyFormatSpeed(int(resp.GetSpeed()))
	eta := common.PrettyFormatDuration(totalSize-downloadedBytes, resp.GetSpeed())
	percentage :=
		fmt.Sprintf("%.2f%%", float64(downloadedBytes)/float64(totalSize)*100)
	return fmt.Sprintf("Downloading... [%s] %s/%s, speed: %s, ETA: %s", percentage, downloadedBytesStr, totalSizeStr, speed, eta)
}
