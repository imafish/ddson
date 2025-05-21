package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"internal/pb"
)

func (s *server) Download(req *pb.DownloadRequest, stream pb.DDSONService_DownloadServer) error {
	log.Printf("Received download request: URL=%s, Version=%s", req.GetUrl(), req.GetVersion())

	// the download request must be from a client in the list
	// TODO: matching client with name is not accurate, improve later
	_, exists := s.clients.getClientByName(req.GetName())
	if !exists {
		log.Printf("Client %s not found in the list", req.GetName())
		err := fmt.Errorf("client %s not found in the list", req.GetName())
		return err
	}

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
	taskInfo := &taskInfo{
		nameOfClient: req.GetName(),
		downloadUrl:  req.GetUrl(),
		state:        taskState_PENDING,
		subTasks:     make([]subTaskInfo, 0),
		checksum:     req.GetChecksum(),
		stream:       stream,
		done:         make(chan bool),
	}
	s.taskList.addTask(taskInfo)

	// wait for the task to complete
	<-taskInfo.done

	completeFile, err := combine(taskInfo)
	if err != nil {
		log.Printf("Error combining files: %v", err)
		return err
	}

	if taskInfo.checksum != "" {
		err = stream.Send(&pb.DownloadStatus{
			Status:  pb.DownloadStatusType_VALIDATING,
			Message: "Validating integrity...",
		})
		if err != nil {
			log.Printf("Failed to send validation status: %v", err)
			return err
		}
		err = validateFile(completeFile, taskInfo.checksum)
		if err != nil {
			log.Printf("Error validating file: %v", err)
			return err
		}
	}

	// send file content
	file, err := os.Open(completeFile)
	if err != nil {
		log.Printf("Error opening file: %v", err)
		return err
	}
	buffer := make([]byte, 1024*1024) // 1 MB buffer

	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error reading file: %v", err)
			return err
		}
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
