package main

import (
	"fmt"
	"sync"
	"time"

	"internal/pb"
)

type clientList struct {
	clients         map[int]*clientInfo
	freeId          int
	mtx             *sync.Mutex
	cond            *sync.Cond
	declinedClients map[string]time.Time
}

func newClientList() *clientList {
	mtx := &sync.Mutex{}
	return &clientList{
		clients:         make(map[int]*clientInfo),
		freeId:          0,
		mtx:             mtx,
		cond:            sync.NewCond(mtx),
		declinedClients: make(map[string]time.Time),
	}
}

func (c *clientList) addClient(name string, version string, clientAddr string, clientPort int) int {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	newId := c.freeId
	c.freeId++
	newClient := newClientInfo(name, newId, version, clientAddr, clientPort)
	c.clients[newId] = newClient
	c.cond.Broadcast() // Notify any waiting goroutines that a new client has been added
	return newId
}

func (c *clientList) getIdleClient() *clientInfo {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	idleClient := c.getIdleClientNoLock()
	for idleClient == nil {
		c.cond.Wait() // Wait for a client to become idle
		idleClient = c.getIdleClientNoLock()
	}

	idleClient.state = pb.ClientState_BUSY
	return idleClient
}

func (c *clientList) getIdleClientNoLock() *clientInfo {
	for _, client := range c.clients {
		if client.state == pb.ClientState_IDLE {
			return client
		}
	}
	return nil
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
	c.removeAndCloseClientNoLock(id)
}

func (c *clientList) banClient(client *clientInfo, seconds int) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.banClientNoLock(client, seconds)
}

func (c *clientList) clientAllowed(name string) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if banUntil, exists := c.declinedClients[name]; exists {
		if time.Now().Before(banUntil) {
			return fmt.Errorf("Client %s banned until %s", name, banUntil) // Client is still banned
		}
		delete(c.declinedClients, name) // Remove from declined list if enough time has passed
	}
	return nil // Client is allowed
}

func (c *clientList) banClientNoLock(client *clientInfo, seconds int) {
	banUntil := time.Now().Add(time.Duration(seconds) * time.Second)
	c.declinedClients[client.name] = banUntil
}

func (c *clientList) removeAndCloseClientNoLock(id int) {
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

func (c *clientList) releaseClient(client *clientInfo) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	client.state = pb.ClientState_IDLE
	client.runningTask = nil
	c.cond.Broadcast() // Notify any waiting goroutines that a client is now idle
}
