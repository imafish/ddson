package main

import (
	"log"
	"time"

	"internal/pb"
)

type clientInfo struct {
	name        string
	id          int32
	version     string
	state       pb.ClientState
	lastSeen    time.Time
	taskChan    chan *subTaskInfo
	runningTask *taskInfo
	taskDone    chan error
	messageChan chan *pb.HeartbeatRequest
	done        chan bool

	// stream      pb.DDSONService_RegisterServer
}

func newClientInfo(name string, id int32, version string, stream pb.DDSONService_RegisterServer) *clientInfo {
	return &clientInfo{
		name:        name,
		version:     version,
		id:          id,
		state:       pb.ClientState_IDLE,
		runningTask: nil,
		lastSeen:    time.Now(),
		taskChan:    make(chan *subTaskInfo, 2),
		taskDone:    make(chan error, 2),
		done:        make(chan bool, 2),

		// stream:      stream,
	}
}

func (c *clientInfo) close() {
	c.done <- true
}

func (client *clientInfo) handleRegistration(req *pb.RegisterRequest, stream pb.DDSONService_RegisterServer, server *server) error {
	defer client.close()

	// Keep the connection open
	for {
		select {
		case <-stream.Context().Done():
			log.Printf("Client %s disconnected, removing from client list", req.Name)
			server.clients.removeClient(req.Name)
			return nil

		case <-client.done:
			log.Printf("client %s is marked closed, exit loop", req.Name)
			return nil

		case subTask := <-client.taskChan:
			log.Printf("Got task for client %s: %v", req.Name, subTask)
			msg := &pb.ServerMessage{
				Type:      pb.ServerMessageType_TASK,
				Timestamp: time.Now().Unix(),
				Url:       subTask.downloadUrl,
				Offset:    subTask.offset,
				Size:      subTask.downloadSize,
				Id:        int32(subTask.id),
			}
			if err := stream.Send(msg); err != nil {
				log.Printf("Failed to send message to %s: %v", req.Name, err)
				server.clients.removeClient(req.Name)
				return err
			}
		}
	}
}
