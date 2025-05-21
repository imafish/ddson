package main

import (
	"time"

	"internal/pb"
)

type clientInfo struct {
	name      string
	version   string
	state     pb.ClientState
	lastSeen  time.Time
	stream    pb.DDSONService_RegisterServer
	connected bool
}

func (c *clientInfo) close() {
	// Close the stream and mark the client as disconnected
	c.connected = false
}
