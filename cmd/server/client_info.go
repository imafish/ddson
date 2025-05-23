package main

import (
	"time"

	"internal/pb"
)

type clientInfo struct {
	name    string
	id      int32
	version string
	addr    string
	port    int

	state       pb.ClientState
	lastSeen    time.Time
	runningTask *taskInfo

	taskChan     chan *subTaskInfo         // used to receive subtasks
	taskDoneChan chan error                // used to signal task completion
	messageChan  chan *pb.HeartbeatRequest // used to send task progress
	quitChan     chan bool                 // used to signal threads to quit
}

func newClientInfo(name string, id int, version string, addr string, port int) *clientInfo {
	return &clientInfo{
		name:    name,
		version: version,
		id:      int32(id),
		addr:    addr,
		port:    port,

		state:       pb.ClientState_IDLE,
		lastSeen:    time.Now(),
		runningTask: nil,

		taskChan:     make(chan *subTaskInfo, 2),
		taskDoneChan: make(chan error, 2),
		messageChan:  make(chan *pb.HeartbeatRequest, 2),
		quitChan:     make(chan bool, 2),
	}
}

// TODO: this should do more things, such as notifying waiting threads, remove itself from the list, etc.
// Consider moving it to clientList
func (c *clientInfo) close() {
	c.quitChan <- true
}
