package main

import (
	"sync"

	"internal/pb"
)

type clientList struct {
	clients map[int]*clientInfo
	freeId  int
	mtx     *sync.Mutex
	cond    *sync.Cond
}

func newClientList() *clientList {
	mtx := &sync.Mutex{}
	return &clientList{
		clients: make(map[int]*clientInfo),
		freeId:  0,
		mtx:     mtx,
		cond:    sync.NewCond(mtx),
	}
}

func (c *clientList) addClient(name string, version string, clientAddr string, clientPort int) int {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	newId := c.freeId
	c.freeId++
	newClient := newClientInfo(name, newId, version, clientAddr, clientPort)
	c.clients[newId] = newClient
	return newId
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

func (c *clientList) getClientById(id int) (*clientInfo, bool) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	client, exists := c.clients[id]
	return client, exists
}

func (c *clientList) removeAndCloseClient(id int) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if client, exists := c.clients[id]; exists {
		client.close()
		delete(c.clients, id)
	}
}

func (c *clientList) getClientByName(name string) (*clientInfo, bool) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for _, client := range c.clients {
		if client.name == name {
			return client, true
		}
	}
	return nil, false
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

func (c *clientList) releaseClient(client *clientInfo) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	client.state = pb.ClientState_IDLE
}
