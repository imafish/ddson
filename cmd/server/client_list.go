package main

import (
	"sync"

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

func (c *clientList) addClient(name string, version string, stream pb.DDSONService_RegisterServer, id int32) *clientInfo {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	newClient := newClientInfo(name, id, version, stream)
	c.clients[name] = newClient
	return newClient
}

func (c *clientList) getIdleClient() *clientInfo {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	var idleClient *clientInfo
	for idleClient == nil {
		for _, client := range c.clients {
			if client.state == pb.ClientState_IDLE {
				idleClient = client
				break
			}
		}
	}
	return idleClient
}

func (c *clientList) getClientById(id int32) (*clientInfo, bool) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for _, client := range c.clients {
		if client.id == id {
			return client, true
		}
	}
	return nil, false
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
