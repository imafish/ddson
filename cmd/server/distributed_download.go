package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

func executeTask(task *taskInfo) {
	// TODO: implement the task execution logic
	// Simulate downloading a file
	log.Printf("Starting download for task %s", task.nameOfClient)
	time.Sleep(5 * time.Second) // Simulate download time
	task.state = taskState_DOWNLOADING
	log.Printf("Download completed for task %s", task.nameOfClient)

	task.done <- true // Signal that the task is done
}

func combine(task *taskInfo) (string, error) {
	// TODO: implement the file combining logic
	// Simulate combining files
	log.Printf("Combining files for task %s", task.nameOfClient)
	time.Sleep(2 * time.Second) // Simulate combine time
	task.state = taskState_COMPLETED
	log.Printf("Files combined for task %s", task.nameOfClient)

	return "combined_file", nil
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
