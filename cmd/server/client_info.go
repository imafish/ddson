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

	messageChan chan int  // used to send task progress // TODO: change to a more specific type
	quitChan    chan bool // used to signal threads to quit
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

		messageChan: make(chan int, 2),
		quitChan:    make(chan bool, 2),
	}
}

// TODO: this should do more things, such as notifying waiting threads, remove itself from the list, etc.
// Consider moving it to clientList
func (c *clientInfo) close() {
	c.quitChan <- true
}
