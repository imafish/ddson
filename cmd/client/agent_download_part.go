package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"internal/common"
	"internal/httputil"
	"internal/pb"
)

// DownloadPart implements the DownloadPart method of the DDSONServiceClientServer interface.
func (c *client) DownloadPart(grpcRequest *pb.DownloadPartRequest, stream pb.DDSONServiceClient_DownloadPartServer) error {
	url, offset, size, clientId, subtaskId := grpcRequest.Url, grpcRequest.Offset, grpcRequest.Size, grpcRequest.ClientId, grpcRequest.SubtaskId
	slog.Info("Received download request", "URL", url, "Offset", offset, "Size", size, "ClientId", clientId, "SubtaskId", subtaskId)

	// Parse .netrc file for credentials
	username, password, err := httputil.GetDataFromNetrc(url)
	if err != nil {
		slog.Error("Failed to parse .netrc file", "error", err)
		return err
	}

	// Create HTTP request with Range header
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Error("Failed to create HTTP request", "error", err)
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+size-1))
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	// Perform the HTTP request
	slog.Debug("Sending request to URL", "URL", url)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("Failed to download file", "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		slog.Error("Unexpected HTTP status", "status", resp.Status)
		return fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	slog.Info("HTTP response OK, start downloading", "url", url, "status", resp.Status, "offset", offset, "size", size)
	startTime := time.Now()
	fullBuffer, err := downloadFromServer(resp, stream, size)
	if err != nil {
		slog.Error("Failed to download file", "error", err)
		return err
	}
	slog.Info("Download completed", "duration", time.Since(startTime), "speed", common.PrettyFormatSpeed(int(float64(size)/time.Since(startTime).Seconds())), "size", common.PrettyFormatSize(size))

	i := 0
	j := 0
	n := int(size)
	for n > 0 {
		// Send the data in chunks
		chunkSize := 1024 * 1024 // 1 MB chunk size
		if n < chunkSize {
			chunkSize = n
		}
		j = i + chunkSize

		slog.Debug("Uploading bytes to server", "start", i, "end", j)
		err = stream.Send(&pb.DownloadStatus{
			Status: pb.DownloadStatusType_TRANSFERRING,
			Data:   fullBuffer[i:j],
		})
		if err != nil {
			slog.Error("Failed to send upload data", "error", err)
			return err
		}

		n -= chunkSize
		i += chunkSize
	}
	slog.Info("Upload completed", "totalBytes", size)
	return nil
}

// downloadFromServer reads the response body and sends progress updates to the server
// It returns the downloaded data as a byte slice.
// Upon success, it ensures that the downloaded data matches the expected size.
func downloadFromServer(resp *http.Response, stream pb.DDSONServiceClient_DownloadPartServer, size int64) ([]byte, error) {
	progressChannel := make(chan int64, 10)
	defer close(progressChannel)

	// goroutine to handle progress updates
	go func() {
		startTime := time.Now()
		reportTime := time.Now()
		totalDownloaded := int64(0)
		downloadedSinceLastUpdate := int64(0)

		for downloaded := range progressChannel {
			downloadSpeed := 0
			elapsed := time.Since(startTime)
			totalDownloaded += downloaded
			downloadedSinceLastUpdate += downloaded
			if elapsed.Seconds() > 0 {
				downloadSpeed = int(float64(totalDownloaded) / elapsed.Seconds())
			}
			slog.Debug("Download progress", "downloaded", common.PrettyFormatSize(downloaded), "total", common.PrettyFormatSize(totalDownloaded), "speed", common.PrettyFormatSpeed(downloadSpeed))
			// update progress to the server every 2 seconds
			if time.Since(reportTime) > 2*time.Second {
				reportTime = time.Now()
				slog.Debug("Sending progress update to server", "downloaded", common.PrettyFormatSize(downloadedSinceLastUpdate), "speed", common.PrettyFormatSpeed(downloadSpeed))
				err := stream.Send(&pb.DownloadStatus{
					Status:          pb.DownloadStatusType_DOWNLOADING,
					Speed:           int32(downloadSpeed),
					DownloadedBytes: downloadedSinceLastUpdate,
				})
				downloadedSinceLastUpdate = 0
				if err != nil {
					slog.Error("Failed to send progress update", "error", err)
					return
				}
			}
		}
	}()

	// Read the response body into a buffer
	// TODO: later, directly copy the http.Response.Body to the calling client (don't wait for download to complete)
	buffer := make([]byte, 1*1024*1024) // 1 MB buffer
	fullBuffer := make([]byte, size)
	totalDownloaded := int64(0)
	for totalDownloaded < size {
		n, err := resp.Body.Read(buffer)
		if err != nil && err != io.EOF {
			slog.Error("Failed to read response body", "error", err)
			return nil, err
		}
		if n == 0 {
			break
		}

		offset := totalDownloaded
		totalDownloaded += int64(n)
		slog.Debug("Downloaded chunk", "chunkSize", common.PrettyFormatSize(int64(n)), "totalDownloaded", common.PrettyFormatSize(totalDownloaded))
		if totalDownloaded > size {
			slog.Error("Read more data than expected", "downloaded", totalDownloaded, "expected", size)
			return nil, fmt.Errorf("read more data than expected: downloaded: %d, expected: %d", totalDownloaded, size)
		}

		// copy the downloaded data to the full buffer
		copied := copy(fullBuffer[offset:totalDownloaded], buffer[:n])
		if copied != n {
			slog.Error("Failed to copy all data to full buffer", "copied", copied, "expected", n)
			return nil, fmt.Errorf("failed to copy all data to full buffer: copied: %d, expected: %d", copied, n)
		}

		// report progress
		progressChannel <- int64(n)
	}

	slog.Debug("Download completed", "totalDownloaded", totalDownloaded)
	if totalDownloaded != size {
		slog.Error("Read less data than expected", "downloaded", totalDownloaded, "expected", size)
		return nil, fmt.Errorf("read less data than expected: downloaded: %d, expected: %d", totalDownloaded, size)
	}

	return fullBuffer, nil
}
