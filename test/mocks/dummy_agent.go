package mocks

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/imafish/ddson/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DummyAgentConfig configures the dummy agent behavior
type DummyAgentConfig struct {
	ServerAddr        string
	AgentName         string
	Port              int32
	HeartbeatInterval time.Duration
	SimulateFailure   bool
	SimulateSlow      bool
}

// DummyAgent is a mock DDSON agent for testing the server
type DummyAgent struct {
	config          *DummyAgentConfig
	client          pb.DDSONServiceClient
	conn            *grpc.ClientConn
	heartbeatTicker *time.Ticker
	stopChan        chan struct{}
	wg              sync.WaitGroup
	mu              sync.RWMutex
	isRunning       bool
	agentName       string
	clientID        int32
}

// NewDummyAgent creates a new dummy agent
func NewDummyAgent(config *DummyAgentConfig) *DummyAgent {
	if config == nil {
		config = &DummyAgentConfig{
			Port:              50051,
			HeartbeatInterval: 10 * time.Second,
		}
	}

	if config.AgentName == "" {
		config.AgentName = fmt.Sprintf("test-agent-%d", time.Now().UnixNano()%10000)
	}

	return &DummyAgent{
		config:   config,
		stopChan: make(chan struct{}),
	}
}

// Start connects to the server and starts the agent
func (a *DummyAgent) Start() error {
	// Connect to server
	conn, err := grpc.Dial(a.config.ServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %v", err)
	}

	a.conn = conn
	a.client = pb.NewDDSONServiceClient(conn)

	// Register with server
	if err := a.Register(); err != nil {
		return err
	}

	a.mu.Lock()
	a.isRunning = true
	a.mu.Unlock()

	// Start heartbeat
	a.StartHeartbeat(a.config.HeartbeatInterval)

	log.Printf("DummyAgent: Started (Name: %s, ID: %d, Server: %s)", a.agentName, a.clientID, a.config.ServerAddr)

	return nil
}

// Stop stops the agent and closes connections
func (a *DummyAgent) Stop() error {
	a.mu.Lock()
	if !a.isRunning {
		a.mu.Unlock()
		return nil
	}
	a.isRunning = false
	a.mu.Unlock()

	close(a.stopChan)

	if a.heartbeatTicker != nil {
		a.heartbeatTicker.Stop()
	}

	a.wg.Wait()

	if a.conn != nil {
		a.conn.Close()
	}

	log.Printf("DummyAgent: Stopped (Name: %s, ID: %d)", a.agentName, a.clientID)

	return nil
}

// Register registers the agent with the server
func (a *DummyAgent) Register() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.RegisterRequest{
		Name:    a.config.AgentName,
		Version: "0.0.2-dev",
		Port:    a.config.Port,
	}

	resp, err := a.client.Register(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to register: %v", err)
	}

	if !resp.Success {
		return fmt.Errorf("registration failed: %s", resp.Message)
	}

	a.agentName = a.config.AgentName
	a.clientID = resp.Id

	log.Printf("DummyAgent: Registered successfully (Name: %s, ID: %d)", a.agentName, a.clientID)

	return nil
}

// StartHeartbeat starts sending periodic heartbeats
func (a *DummyAgent) StartHeartbeat(interval time.Duration) {
	a.heartbeatTicker = time.NewTicker(interval)

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()

		for {
			select {
			case <-a.stopChan:
				return
			case <-a.heartbeatTicker.C:
				if err := a.sendHeartbeat(); err != nil {
					log.Printf("DummyAgent: Heartbeat error: %v", err)
				}
			}
		}
	}()
}

// sendHeartbeat sends a heartbeat to the server
func (a *DummyAgent) sendHeartbeat() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.HeartbeatRequest{
		Name: a.agentName,
		Id:   a.clientID,
	}

	resp, err := a.client.Heartbeat(ctx, req)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("heartbeat rejected: %s", resp.Message)
	}

	log.Printf("DummyAgent: Heartbeat sent (Name: %s, ID: %d)", a.agentName, a.clientID)

	return nil
}

// ReportProgress reports task progress to the server (helper for testing)
func (a *DummyAgent) ReportProgress(taskID string, progress int32) error {
	// This is a helper method for testing - in real implementation,
	// progress would be reported through the Download streaming RPC
	log.Printf("DummyAgent: Progress reported (Task: %s, Progress: %d%%)", taskID, progress)
	return nil
}

// ReportError reports a task error to the server (helper for testing)
func (a *DummyAgent) ReportError(taskID string, errorMsg string) error {
	// This is a helper method for testing - in real implementation,
	// errors would be reported through the Download streaming RPC
	log.Printf("DummyAgent: Error reported (Task: %s, Error: %s)", taskID, errorMsg)
	return nil
}

// SimulateDownload simulates a download with progress updates to the server
func (a *DummyAgent) SimulateDownload(server *DummyServer, taskID string, duration time.Duration, successRate float64) error {
	steps := 10
	stepDuration := duration / time.Duration(steps)

	for i := 0; i <= steps; i++ {
		progress := int32((i * 100) / steps)

		// Report progress using the stub method
		if err := a.ReportProgress(taskID, progress); err != nil {
			return err
		}

		// Update server task progress
		if server != nil {
			if err := server.UpdateTaskProgress(taskID, progress); err != nil {
				return fmt.Errorf("failed to update server task progress: %w", err)
			}
		}

		if i < steps {
			time.Sleep(stepDuration)
		}
	}

	// Simulate random failure
	if a.config.SimulateFailure {
		if server != nil {
			server.UpdateTaskError(taskID, "Simulated download failure")
		}
		return a.ReportError(taskID, "Simulated download failure")
	}

	return nil
}

// SimulateFailure causes the agent to simulate a failure condition
func (a *DummyAgent) SimulateFailure() {
	a.mu.Lock()
	a.config.SimulateFailure = true
	a.mu.Unlock()

	log.Printf("DummyAgent: Failure mode activated (Name: %s)", a.agentName)
}

// GetAgentID returns the agent ID
func (a *DummyAgent) GetAgentID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.agentName
}

// GetClientID returns the client ID
func (a *DummyAgent) GetClientID() int32 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.clientID
}

// IsRunning returns whether the agent is currently running
func (a *DummyAgent) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isRunning
}
