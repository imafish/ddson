package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"internal/pb"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func executeTask(task *taskInfo, server *server) {
	defer task.markDone()

	// Check if server supports partial downloads
	supportsPartial, totalSize, err := checkPartialDownloadSupport(task.downloadUrl)
	if err != nil {
		log.Printf("Error checking partial download support: %v", err)
		task.setError(err)
		return
	}

	if !supportsPartial {
		log.Printf("Server does not support partial downloads, downloading the whole file")
		err = fmt.Errorf("server does not support partial downloads")
		task.setError(err)
		return
	}

	// create temporary folder in /tmp
	tmpDir, err := os.MkdirTemp("", "ddson")
	if err != nil {
		log.Printf("Error creating temporary directory: %v", err)
		task.setError(err)
		return
	}

	// create sub tasks
	chunkSize := int64(50 * 1024 * 1024) // 1 MB
	i := int32(0)

	for offset := int64(0); offset < totalSize; offset += chunkSize {
		downloadSize := chunkSize
		if offset+downloadSize > totalSize {
			downloadSize = totalSize - offset
		}

		targetFile := fmt.Sprintf("%s/%d", tmpDir, offset)
		subTask := newSubTaskInfo(task.downloadUrl, i, offset, downloadSize, targetFile)
		i++
		task.pendingSubTasks = append(task.pendingSubTasks, subTask)
	}

	totalSubTasks := len(task.pendingSubTasks)
	log.Printf("Created %d sub tasks for task %d", totalSubTasks, task.id)

	// Start downloading each sub task
	task.mtx.Lock()
	for len(task.pendingSubTasks) > 0 {
		subTask := task.pendingSubTasks[0]
		task.pendingSubTasks = task.pendingSubTasks[1:]
		task.mtx.Unlock()

		log.Printf("Starting download subtask #%d, offset: %d, size: %d", subTask.id, subTask.offset, subTask.downloadSize)
		// This blocks until a client is available
		client := server.clients.getIdleClient()
		log.Printf("Got idle client %d for subtask %d", client.id, subTask.id)

		go func() {
			defer server.clients.releaseClient(client)
			defer task.cond.Broadcast()
			client.runningTask = task

			task.mtx.Lock()
			subTask.state = taskState_DOWNLOADING
			task.runningSubTasks[subTask.id] = subTask
			task.mtx.Unlock()
			defer func() {
				task.mtx.Lock()
				subTask.state = taskState_COMPLETED
				delete(task.runningSubTasks, subTask.id)
				task.mtx.Unlock()
			}()

			subTask.assignedTo = client.id

			// TODO: add a timeout for the download
			// TODO: send a message via messageChan to the client, about how many agents are downloading

			err = downloadChunk(subTask, client)
			// TODO retry here?
			if err != nil {
				log.Printf("Error downloading chunk: %v", err)
				task.setError(err)
				return
			}

			task.mtx.Lock()
			task.completedSubTasks = append(task.completedSubTasks, subTask)
			task.mtx.Unlock()
		}()
	}

	// Wait for all sub tasks to complete
	task.mtx.Lock()
	for len(task.completedSubTasks) < totalSubTasks && task.err == nil {
		task.cond.Wait()
	}
	if task.err != nil {
		// don't need to continue if there is an error
		log.Printf("Error in task: %v", task.err)
		task.mtx.Unlock()
		return
	}
	// from now on, this lock is only used here.
	defer task.mtx.Unlock()

	log.Printf("combining %d sub tasks for task %d", totalSubTasks, task.id)
	completeFile, err := combine(task)
	if err != nil {
		log.Printf("Error combining files: %v", err)
		task.setError(err)
		return
	}

	if task.checksum != "" {
		err = task.stream.Send(&pb.DownloadStatus{
			Status:  pb.DownloadStatusType_VALIDATING,
			Message: "Validating integrity...",
		})
		if err != nil {
			log.Printf("Failed to send validation status: %v", err)
			task.setError(err)
			return
		}
		err = validateFile(completeFile, task.checksum)
		if err != nil {
			log.Printf("Error validating file: %v", err)
			task.setError(err)
			return
		}
	} else {
		log.Printf("No checksum provided, skipping validation")
	}

	task.state = taskState_COMPLETED
	task.completeFilePath = completeFile
}

func downloadChunk(subTask *subTaskInfo, client *clientInfo) error {
	downloadUrl, offset, downloadSize := subTask.downloadUrl, subTask.offset, subTask.downloadSize
	log.Printf("Downloading chunk from %s, offset: %d, size: %d, client: #%d", downloadUrl, offset, downloadSize, client.id)

	// TODO: don't initialize a grpc client for each download

	// Create a grpc request to the client to ask for the download
	clientAddr := fmt.Sprintf("%s:%d", client.addr, client.port)
	log.Printf("Connecting to client %d at %s", client.id, clientAddr)
	// Establish a connection to the server
	conn, err := grpc.NewClient(clientAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to server: %v", err)
	}
	defer conn.Close()

	grpcClient := pb.NewDDSONServiceClientClient(conn)
	if err != nil {
		log.Printf("Error creating gRPC client: %v", err)
		return err
	}
	// Send the request to the client
	stream, err := grpcClient.DownloadPart(context.Background(), &pb.DownloadPartRequest{
		Url:    downloadUrl,
		Offset: offset,
		Size:   downloadSize,
	})
	if err != nil {
		log.Printf("Error sending download request: %v", err)
		return err
	}
	defer stream.CloseSend()
	// Read the response from the client
	targetFile := subTask.targetFile
	file, err := os.Create(targetFile)
	if err != nil {
		log.Printf("Error creating file: %v", err)
		return err
	}
	defer file.Close()

	// Read the data from the stream and write it to the file
	log.Printf("Receiving data for subtask #%d", subTask.id)
	var received int64 = 0
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			log.Printf("Download completed for subtask #%d", subTask.id)
			break
		}
		if err != nil {
			log.Printf("Error receiving data: %v", err)
			return err
		}
		if resp.GetStatus() != pb.DownloadStatusType_TRANSFERRING {
			// TODO: handle other statuses
			log.Printf("Unexpected status: %s", resp.GetStatus())
			return fmt.Errorf("unexpected status: %s", resp.GetStatus())
		}
		// Write the data to the file
		n, err := file.Write(resp.GetData())
		if err != nil {
			log.Printf("Error writing to file: %v", err)
			return err
		}
		received += int64(n)
	}
	if received != downloadSize {
		log.Printf("Error: received %d bytes, expected %d bytes", received, downloadSize)
		return fmt.Errorf("received %d bytes, expected %d bytes", received, downloadSize)
	}
	log.Printf("Download completed for subtask #%d, saved to %s", subTask.id, targetFile)
	return nil
}

func combine(task *taskInfo) (string, error) {
	// Create a new file to write the combined content
	combinedFile, err := os.CreateTemp("", "combined_")
	if err != nil {
		log.Printf("Error creating combined file: %v", err)
		return "", err
	}
	defer combinedFile.Close()

	// sort the completed sub tasks by offset
	sort.Slice(task.completedSubTasks, func(i, j int) bool {
		return task.completedSubTasks[i].offset < task.completedSubTasks[j].offset
	})

	currentOffset := int64(0)
	for _, subTask := range task.completedSubTasks {
		if subTask.offset != currentOffset {
			log.Printf("Error: subtask offset %d does not match expected offset %d", subTask.offset, currentOffset)
			return "", fmt.Errorf("subtask offset mismatch: got %d, want %d", subTask.offset, currentOffset)
		}
	}

	for _, subTask := range task.completedSubTasks {
		log.Printf("Combining sub task #%d, offset: %d, size: %d", subTask.id, subTask.offset, subTask.downloadSize)
		// Open the sub task file
		file, err := os.Open(subTask.targetFile)
		if err != nil {
			log.Printf("Error opening sub task file: %v", err)
			return "", err
		}
		defer file.Close()
		// Read the content and write it to the combined file
		_, err = io.Copy(combinedFile, file)
		if err != nil {
			log.Printf("Error writing to combined file: %v", err)
			return "", err
		}
		// Update the current offset
		currentOffset += subTask.downloadSize
	}

	return combinedFile.Name(), nil
}

func validateFile(file string, checksum string) error {
	// calculate the checksum (sha256) of the file
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return err
	}
	sum := hex.EncodeToString(hasher.Sum(nil))
	if sum != checksum {
		return fmt.Errorf("checksum mismatch: got %s, want %s", sum, checksum)
	}
	return nil
}

func checkPartialDownloadSupport(url string) (bool, int64, error) {
	if url == "" {
		return false, 0, fmt.Errorf("invalid URL")
	}

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false, 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()

	supportsPartial := resp.Header.Get("Accept-Ranges") == "bytes"
	totalSize, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return false, 0, err
	}

	return supportsPartial, totalSize, nil
}
