package main

import (
	"time"

	"internal/pb"
)

type clientInfo struct {
	name     string
	version  string
	state    pb.ClientState
	lastSeen time.Time
	stream   pb.DDSONService_RegisterServer
	done     chan bool
}

func (c *clientInfo) close() {
	c.done <- true
}
