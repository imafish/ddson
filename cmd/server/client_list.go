package main

import (
	"sync"
	"time"

	"internal/pb"
)

type clientList struct {
	clients map[string]*clientInfo
	mtx     sync.Mutex
}

func newClientList() *clientList {
	return &clientList{
		clients: make(map[string]*clientInfo),
	}
}

func (c *clientList) addClient(name string, version string, stream pb.DDSONService_RegisterServer) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.clients[name] = &clientInfo{
		name:     name,
		version:  version,
		state:    pb.ClientState_IDLE,
		lastSeen: time.Now(),
		stream:   stream,
		done:     make(chan bool, 2),
	}
}

func (c *clientList) removeClient(name string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if client, exists := c.clients[name]; exists {
		client.close()
		delete(c.clients, name)
	}
}

func (c *clientList) getClientByName(name string) (*clientInfo, bool) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	client, exists := c.clients[name]
	return client, exists
}

func (c *clientList) getOneIdleClient() (*clientInfo, bool) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for _, client := range c.clients {
		if client.state == pb.ClientState_IDLE {
			return client, true
		}
	}
	return nil, false
}
