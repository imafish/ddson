package mocks

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/imafish/ddson/internal/pb"
	"google.golang.org/grpc"
)

// DummyServerConfig configures the dummy server behavior
type DummyServerConfig struct {
	Port               int
	HeartbeatTimeout   time.Duration
	SimulateErrors     bool
	TaskAssignmentMode string // "round-robin", "random", "capacity"
	AgentBanThreshold  int    // Number of task failures before an agent is banned
}

// DummyServer is a mock DDSON server for testing agents
type DummyServer struct {
	pb.UnimplementedDDSONServiceServer
	config           *DummyServerConfig
	grpcServer       *grpc.Server
	listener         net.Listener
	registeredAgents map[string]*AgentInfo
	idToName         map[string]string // Maps numeric ID to agent name
	assignedTasks    map[string]*TaskInfo
	heartbeats       map[string]time.Time
	agentFailures    map[string]int
	mu               sync.RWMutex
	taskCounter      int32
	clientIDCounter  int32
	addr             string
}

// AgentInfo stores information about a registered agent
type AgentInfo struct {
	ID            string
	Address       string
	Port          int32
	Capacity      int32
	RegisteredAt  time.Time
	LastHeartbeat time.Time
	AssignedTasks []string
	Status        string // "healthy", "unhealthy", "offline", "banned"
}

// TaskInfo stores information about a task
type TaskInfo struct {
	ID          string
	URL         string
	AssignedTo  string
	Status      string // "pending", "assigned", "downloading", "completed", "failed"
	AssignedAt  time.Time
	CompletedAt time.Time
	Progress    int32
	Error       string
}

// NewDummyServer creates a new dummy DDSON server
func NewDummyServer(config *DummyServerConfig) *DummyServer {
	if config == nil {
		config = &DummyServerConfig{
			Port:               0,
			HeartbeatTimeout:   30 * time.Second,
			TaskAssignmentMode: "round-robin",
			AgentBanThreshold:  3,
		}
	}

	if config.AgentBanThreshold == 0 {
		config.AgentBanThreshold = 3
	}

	return &DummyServer{
		config:           config,
		registeredAgents: make(map[string]*AgentInfo),
		idToName:         make(map[string]string),
		assignedTasks:    make(map[string]*TaskInfo),
		heartbeats:       make(map[string]time.Time),
		agentFailures:    make(map[string]int),
	}
}

// Start starts the gRPC server
func (s *DummyServer) Start() error {
	listener, err := listenOnPort(s.config.Port)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	s.listener = listener
	s.addr = listener.Addr().String()

	s.grpcServer = grpc.NewServer()
	pb.RegisterDDSONServiceServer(s.grpcServer, s)

	log.Printf("DummyServer: Starting on %s", s.addr)

	go func() {
		if err := s.grpcServer.Serve(listener); err != nil {
			log.Printf("DummyServer error: %v", err)
		}
	}()

	// Start heartbeat monitor
	go s.monitorHeartbeats()

	return nil
}

// Stop stops the gRPC server
func (s *DummyServer) Stop() {
	log.Printf("DummyServer: Stopping")
	if s.grpcServer != nil {
		s.grpcServer.Stop()
	}
}

// Addr returns the server address
func (s *DummyServer) Addr() string {
	return s.addr
}

// RegisterAgent implements the gRPC Register method (renamed for clarity)
func (s *DummyServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate client ID
	clientID := atomic.AddInt32(&s.clientIDCounter, 1)
	agentName := req.Name
	if agentName == "" {
		agentName = fmt.Sprintf("agent-%d", clientID)
	}

	log.Printf("DummyServer: Registering agent %s (ID: %d) at port %d", agentName, clientID, req.Port)

	agent := &AgentInfo{
		ID:            agentName,
		Address:       "127.0.0.1",
		Port:          req.Port,
		Capacity:      5, // Default capacity
		RegisteredAt:  time.Now(),
		LastHeartbeat: time.Now(),
		AssignedTasks: make([]string, 0),
		Status:        "healthy",
	}

	s.registeredAgents[agentName] = agent
	s.idToName[fmt.Sprintf("%d", clientID)] = agentName
	s.heartbeats[agentName] = time.Now()

	return &pb.RegisterResponse{
		Success:       true,
		Message:       "Agent registered successfully",
		Id:            clientID,
		ServerVersion: "0.0.2-dev",
	}, nil
}

// SendHeartbeat implements the gRPC Heartbeat method
func (s *DummyServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agentName := req.Name
	agent, exists := s.registeredAgents[agentName]
	if !exists {
		return &pb.HeartbeatResponse{
			Success: false,
			Message: "Agent not registered",
		}, nil
	}

	if agent.Status == "banned" {
		return &pb.HeartbeatResponse{
			Success: false,
			Message: "Agent is banned",
		}, nil
	}

	agent.LastHeartbeat = time.Now()
	agent.Status = "healthy"
	s.heartbeats[agentName] = time.Now()

	log.Printf("DummyServer: Heartbeat from agent %s (ID: %d)", agentName, req.Id)

	return &pb.HeartbeatResponse{
		Success: true,
		Message: "Heartbeat received",
	}, nil
}

// AssignTask assigns a task to an agent (test helper method)
func (s *DummyServer) AssignTask(agentID string, taskID string, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Try to resolve numeric ID to agent name
	agentKey := agentID
	if agentName, exists := s.idToName[agentID]; exists {
		agentKey = agentName
	}

	agent, exists := s.registeredAgents[agentKey]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	if agent.Status != "healthy" {
		return fmt.Errorf("agent is not eligible for assignment: %s (%s)", agentID, agent.Status)
	}

	task := &TaskInfo{
		ID:         taskID,
		URL:        url,
		AssignedTo: agentID,
		Status:     "assigned",
		AssignedAt: time.Now(),
	}

	s.assignedTasks[taskID] = task
	agent.AssignedTasks = append(agent.AssignedTasks, taskID)

	log.Printf("DummyServer: Assigned task %s to agent %s (URL: %s)", taskID, agentID, url)

	return nil
}

// UpdateTaskProgress updates task progress (test helper)
func (s *DummyServer) UpdateTaskProgress(taskID string, progress int32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.assignedTasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Progress = progress
	if progress >= 100 {
		task.Status = "completed"
		task.CompletedAt = time.Now()
	} else {
		task.Status = "downloading"
	}

	log.Printf("DummyServer: Task %s progress: %d%%", taskID, progress)
	return nil
}

// UpdateTaskError updates task error status (test helper)
func (s *DummyServer) UpdateTaskError(taskID string, errorMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.assignedTasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Status = "failed"
	task.Error = errorMsg
	task.CompletedAt = time.Now()

	agentID := task.AssignedTo
	if agentID != "" {
		s.agentFailures[agentID]++
		failureCount := s.agentFailures[agentID]

		if s.config.AgentBanThreshold > 0 && failureCount >= s.config.AgentBanThreshold {
			if agent, exists := s.registeredAgents[agentID]; exists {
				agent.Status = "banned"
				log.Printf("DummyServer: Agent %s banned after %d task failures", agentID, failureCount)
			}
		}
	}

	log.Printf("DummyServer: Task %s failed: %s", taskID, errorMsg)
	return nil
}

// GetAgentFailureCount returns tracked task failure count for an agent
func (s *DummyServer) GetAgentFailureCount(agentID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.agentFailures[agentID]
}

// ReassignTaskToAnyHealthyAgent reassigns a task to a healthy agent other than the current assignee
func (s *DummyServer) ReassignTaskToAnyHealthyAgent(taskID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.assignedTasks[taskID]
	if !exists {
		return "", fmt.Errorf("task not found: %s", taskID)
	}

	currentAgentID := task.AssignedTo
	for agentID, agent := range s.registeredAgents {
		if agentID == currentAgentID {
			continue
		}
		if agent.Status == "healthy" {
			task.AssignedTo = agentID
			task.Status = "assigned"
			task.Progress = 0
			task.Error = ""
			task.CompletedAt = time.Time{}
			agent.AssignedTasks = append(agent.AssignedTasks, taskID)
			log.Printf("DummyServer: Reassigned task %s from %s to %s", taskID, currentAgentID, agentID)
			return agentID, nil
		}
	}

	return "", fmt.Errorf("no healthy agent available to reassign task %s", taskID)
}

// GetAgents returns all registered agents
func (s *DummyServer) GetAgents() []*AgentInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]*AgentInfo, 0, len(s.registeredAgents))
	for _, agent := range s.registeredAgents {
		agentCopy := *agent
		agents = append(agents, &agentCopy)
	}

	return agents
}

// GetAgent returns a specific agent by ID
func (s *DummyServer) GetAgent(agentID string) (*AgentInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agent, exists := s.registeredAgents[agentID]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	agentCopy := *agent
	return &agentCopy, nil
}

// GetTask returns a specific task by ID
func (s *DummyServer) GetTask(taskID string) (*TaskInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.assignedTasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	taskCopy := *task
	return &taskCopy, nil
}

// GetHealthyAgents returns all healthy agents
func (s *DummyServer) GetHealthyAgents() []*AgentInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]*AgentInfo, 0)
	for _, agent := range s.registeredAgents {
		if agent.Status == "healthy" {
			agentCopy := *agent
			agents = append(agents, &agentCopy)
		}
	}

	return agents
}

// monitorHeartbeats monitors agent heartbeats and marks unhealthy agents
func (s *DummyServer) monitorHeartbeats() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()

		for agentID, lastHeartbeat := range s.heartbeats {
			if now.Sub(lastHeartbeat) > s.config.HeartbeatTimeout {
				if agent, exists := s.registeredAgents[agentID]; exists {
					if agent.Status == "healthy" {
						log.Printf("DummyServer: Agent %s marked as unhealthy (no heartbeat for %v)", agentID, now.Sub(lastHeartbeat))
						agent.Status = "unhealthy"
					}
				}
			}
		}

		s.mu.Unlock()
	}
}

// WaitForAgents waits for a specific number of agents to register
func (s *DummyServer) WaitForAgents(count int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		s.mu.RLock()
		currentCount := len(s.registeredAgents)
		s.mu.RUnlock()

		if currentCount >= count {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for %d agents (got %d)", count, currentCount)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// WaitForTaskCompletion waits for a task to complete
func (s *DummyServer) WaitForTaskCompletion(taskID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		task, err := s.GetTask(taskID)
		if err != nil {
			return err
		}

		if task.Status == "completed" {
			return nil
		}

		if task.Status == "failed" {
			return fmt.Errorf("task failed: %s", task.Error)
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for task completion (status: %s)", task.Status)
		}

		time.Sleep(100 * time.Millisecond)
	}
}
