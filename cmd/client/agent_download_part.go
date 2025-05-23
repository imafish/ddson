package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/bgentry/go-netrc/netrc"

	"internal/pb"
)

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
