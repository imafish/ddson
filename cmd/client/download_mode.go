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
	"internal/progressbar"
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
	var resp *pb.DownloadStatus
	bottomLineFunc := func(percentage float64, width int) string {
		return fmtProgress(resp, totalSize)
	}
	progressBar, err := progressbar.New(progressbar.Basketball(), os.Stdout, bottomLineFunc)
	if err != nil {
		slog.Error("Failed to create progress bar", "error", err)
	} else {
		progressBar.Start()
		defer progressBar.Done()
	}

	// Process the responses from the server
	for {
		resp, err = stream.Recv()

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
				if progressBar != nil {
					progressBar.Update(1.0)
				} else {
					slog.Info("Download completed")
				}
				break
			}
			slog.Error("Error receiving data", "error", err)
			os.Exit(1)
		}

		printProgress(resp, totalSize, progressBar)
		switch resp.GetStatus() {
		case pb.DownloadStatusType_DOWNLOADING:
			if progressBar != nil {
				progressBar.Update(float64(resp.GetTotalDownloadedBytes()) / float64(totalSize))
			} else {
				slog.Info(fmtProgress(resp, totalSize))
			}

		case pb.DownloadStatusType_TRANSFERRING:
			// Write data to the file
			if _, err := file.Write(resp.GetData()); err != nil {
				slog.Error("Failed to write data to file", "error", err)
				os.Exit(1)
			}
			received += int64(len(resp.GetData()))
		}
	}

	slog.Info("Download completed", "file", *output, "size", common.PrettyFormatSize(received))
}

func printProgress(resp *pb.DownloadStatus, totalSize int64, progressBar *progressbar.ProgressBar) {
	if resp == nil {
		return
	}

	if progressBar != nil {
		progressBar.Update(float64(resp.GetTotalDownloadedBytes()) / float64(totalSize))
	} else {
		slog.Info(fmtProgress(resp, totalSize))
	}
}

func fmtProgress(resp *pb.DownloadStatus, totalSize int64) string {
	if resp == nil {
		return "..."
	}

	switch resp.GetStatus() {
	case pb.DownloadStatusType_PENDING:
		return fmt.Sprintf("Pending... %d in queue, %d clients connected, message: %s", resp.GetNumberInQueue(), resp.GetClientCount(), resp.GetMessage())

	case pb.DownloadStatusType_VALIDATING:
		return "Validating..."

	case pb.DownloadStatusType_TRANSFERRING:
		return fmt.Sprintf("Transferring... %s/%s", common.PrettyFormatSize(resp.GetTotalDownloadedBytes()), common.PrettyFormatSize(totalSize))

	case pb.DownloadStatusType_DOWNLOADING:
		downloadedBytes := resp.GetTotalDownloadedBytes()
		downloadedBytesStr := common.PrettyFormatSize(downloadedBytes)
		totalSizeStr := common.PrettyFormatSize(totalSize)
		speed := common.PrettyFormatSpeed(int(resp.GetSpeed()))
		eta := common.PrettyFormatDuration(totalSize-downloadedBytes, resp.GetSpeed())
		percentage :=
			fmt.Sprintf("%.2f%%", float64(downloadedBytes)/float64(totalSize)*100)
		return fmt.Sprintf("Downloading... [%s] %s/%s, speed: %s, ETA: %s", percentage, downloadedBytesStr, totalSizeStr, speed, eta)

	default:
		return fmt.Sprintf("Status: %s", resp.GetStatus().String())
	}
}
