package main

import (
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
		err = fmt.Errorf("Server does not support partial downloads")
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

	log.Printf("Created %d sub tasks for task %s", len(task.pendingSubTasks), task.nameOfClient)

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
			client.runningTask = task
			defer func() {
				client.runningTask = nil
			}()

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
	}

	task.state = taskState_COMPLETED
	task.completeFilePath = completeFile
}

func downloadChunk(subTask *subTaskInfo, client *clientInfo) error {
	client.taskChan <- subTask

	// Wait for the client to finish downloading
	for {
		select {
		case msg := <-client.messageChan:
			// TODO: gather the progress and update the sub task state
			log.Printf("Client %d reported progress for subtask %d: %s", client.id, 7777, msg.GetName())

		case err := <-client.taskDone:
			if err != nil {
				log.Printf("Error downloading chunk: %v", err)
				return err
			}
			log.Printf("Client %d finished downloading subtask %d", client.id, subTask.id)
			return nil
		}
	}
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
