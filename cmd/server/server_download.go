package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"internal/pb"
)

func (s *server) Download(req *pb.DownloadRequest, stream pb.DDSONService_DownloadServer) error {
	log.Printf("Received download request: URL=%s, id=%d", req.GetUrl(), req.GetClientId())

	// the download request must be from a client in the list
	// TODO: matching client with name is not accurate, improve later
	log.Printf("NOT checking client id for now. implement later")
	clientId := 0
	// _, exists := s.clients.getClientById(int(req.GetClientId()))
	// if !exists {
	// 	log.Printf("Client #%d not found in the list", req.GetClientId())
	// 	err := fmt.Errorf("client #%d not found in the list", req.GetClientId())
	// 	return err
	// }

	// Send initial status as PENDING
	err := stream.Send(&pb.DownloadStatus{
		Status:   pb.DownloadStatusType_PENDING,
		Progress: int32(s.taskList.size()),
		Message:  "Download request is being processed",
	})
	if err != nil {
		log.Printf("Failed to send initial status: %v", err)
		return err
	}

	// Create a task and add it to task list
	taskInfo := s.taskList.addTask(req.GetUrl(), req.GetChecksum(), stream, clientId)

	// wait for the task to complete
	// TODO: periodically update the status (using select?)
	<-taskInfo.done

	// send file content
	if taskInfo.err != nil {
		log.Printf("Error in task: %v", taskInfo.err)
		return taskInfo.err
	}
	if taskInfo.completeFilePath == "" {
		log.Printf("No file to send for task %d", taskInfo.id)
		return fmt.Errorf("no file to send for task %d", taskInfo.id)
	}

	file, err := os.Open(taskInfo.completeFilePath)
	if err != nil {
		log.Printf("Error opening file: %v", err)
		return err
	}
	defer file.Close()
	defer os.Remove(taskInfo.completeFilePath) // Clean up after sending

	fileStat, err := file.Stat()
	if err != nil {
		log.Printf("Error getting file info: %v", err)
		return err
	}
	fileSize := fileStat.Size()

	log.Printf("Sending file: %s, size=%d", taskInfo.completeFilePath, fileSize)
	buffer := make([]byte, 1024*1024) // 1 MB buffer
	totalBytesSent := 0
	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error reading file: %v", err)
			return err
		}
		log.Printf("Sending %d bytes, total sent: %d", n, totalBytesSent)
		err = stream.Send(&pb.DownloadStatus{
			Status: pb.DownloadStatusType_TRANSFERRING,
			Data:   buffer[:n],
		})
		if err != nil {
			log.Printf("Error sending file data: %v", err)
			return err
		}
	}

	log.Printf("Download completed: URL=%s", req.GetUrl())
	return nil
}
