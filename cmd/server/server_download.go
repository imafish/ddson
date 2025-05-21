package main

import (
	"io"
	"log"
	"os"
	"time"

	"internal/pb"
)

func (s *server) Download(req *pb.DownloadRequest, stream pb.DDSONService_DownloadServer) error {
	log.Printf("Received download request: URL=%s, Version=%s", req.GetUrl(), req.GetVersion())

	// Send initial status as PENDING
	err := stream.Send(&pb.DownloadStatus{
		Status:   pb.DownloadStatusType_PENDING,
		Progress: 2,
		Message:  "Download request is being processed",
	})
	if err != nil {
		log.Printf("Failed to send initial status: %v", err)
		return err
	}

	for i := 0; i < 5; i++ {
		log.Printf("Simulating processing...")
		time.Sleep(1 * time.Second)
		err = stream.Send(&pb.DownloadStatus{
			Status:     pb.DownloadStatusType_DOWNLOADING,
			Downloaded: int64(i * 2000),
			Total:      10000,
			Speed:      2000,
		})
		if err != nil {
			log.Printf("Failed to send processing status: %v", err)
			return err
		}
	}

	// simuate validation
	err = stream.Send(&pb.DownloadStatus{
		Status:  pb.DownloadStatusType_VALIDATING,
		Message: "Validating integrity...",
	})
	if err != nil {
		log.Printf("Failed to send validation status: %v", err)
		return err
	}
	// Simulate validation delay
	time.Sleep(2 * time.Second)

	// send file content
	file, err := os.Open("/home/ubuntu/workspace_bazel_prefetcher/data/content_addressable/sha256/da762fc50ba1464a494a6581a848ee92661b1f390c211f4737d8d84210c986a7/file")
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
			Status: pb.DownloadStatusType_TRANSFERING,
			Data:   buffer[:n],
		})
		if err != nil {
			log.Printf("Error sending file data: %v", err)
			return err
		}
	}

	// Send final status as COMPLETED
	err = stream.Send(&pb.DownloadStatus{
		Status:  pb.DownloadStatusType_COMPLETED,
		Message: "Download completed successfully",
	})
	if err != nil {
		log.Printf("Failed to send completed status: %v", err)
		return err
	}

	log.Printf("Download completed: URL=%s", req.GetUrl())
	return nil
}
