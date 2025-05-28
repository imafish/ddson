package main

import (
	"time"

	"internal/pb"
)

type clientInfo struct {
	name    string
	id      int
	version string
	addr    string
	port    int

	state       pb.ClientState
	lastSeen    time.Time
	runningTask *taskInfo
	errCount    int

	messageChan chan int  // used to send task progress // TODO: not used yet, consider removing
	quitChan    chan bool // used to signal threads to quit // TODO: not used yet, consider removing
}

func newClientInfo(name string, id int, version string, addr string, port int) *clientInfo {
	return &clientInfo{
		name:    name,
		version: version,
		id:      id,
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
