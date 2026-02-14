package server

import (
	"testing"
	"time"

	"github.com/imafish/ddson/test/helpers"
	"github.com/imafish/ddson/test/mocks"
)

// TestServerAgentRegistration tests agent registration handling
func TestServerAgentRegistration(t *testing.T) {
	config := &mocks.DummyServerConfig{
		Port:             0,
		HeartbeatTimeout: 30 * time.Second,
	}

	server := mocks.NewDummyServer(config)

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	t.Logf("Dummy server started at: %s", server.Addr())

	// Create and start a dummy agent
	agentConfig := &mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		Port:              50051,
		HeartbeatInterval: 10 * time.Second,
	}

	agent := mocks.NewDummyAgent(agentConfig)

	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy agent")
	defer agent.Stop()

	// Wait for registration
	time.Sleep(500 * time.Millisecond)

	// Verify agent is registered
	agents := server.GetAgents()
	helpers.AssertEqual(t, 1, len(agents), "Expected 1 registered agent")

	if len(agents) > 0 {
		t.Logf("Registered agent: ID=%s, Status=%s", agents[0].ID, agents[0].Status)
		helpers.AssertEqual(t, "healthy", agents[0].Status, "Agent should be healthy")
	}
}

// TestServerMultipleAgentRegistration tests multiple agents registering
func TestServerMultipleAgentRegistration(t *testing.T) {
	server := mocks.NewDummyServer(nil)

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	numAgents := 3
	agents := make([]*mocks.DummyAgent, numAgents)

	// Start multiple agents
	for i := 0; i < numAgents; i++ {
		agentConfig := &mocks.DummyAgentConfig{
			ServerAddr:        server.Addr(),
			Port:              int32(50051 + i),
			HeartbeatInterval: 5 * time.Second,
		}

		agents[i] = mocks.NewDummyAgent(agentConfig)
		err := agents[i].Start()
		helpers.AssertNoError(t, err, "Failed to start agent")
		defer agents[i].Stop()
	}

	// Wait for all agents to register
	err = server.WaitForAgents(numAgents, 5*time.Second)
	helpers.AssertNoError(t, err, "Failed to wait for agents")

	// Verify all agents are registered
	registeredAgents := server.GetAgents()
	helpers.AssertEqual(t, numAgents, len(registeredAgents), "Expected all agents to be registered")

	for i, agent := range registeredAgents {
		t.Logf("Agent %d: ID=%s, Port=%d, Status=%s", i+1, agent.ID, agent.Port, agent.Status)
	}
}

// TestServerHeartbeatManagement tests heartbeat handling
func TestServerHeartbeatManagement(t *testing.T) {
	config := &mocks.DummyServerConfig{
		Port:             0,
		HeartbeatTimeout: 2 * time.Second, // Short timeout for testing
	}

	server := mocks.NewDummyServer(config)

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	// Start agent with heartbeats
	agentConfig := &mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		HeartbeatInterval: 500 * time.Millisecond,
	}

	agent := mocks.NewDummyAgent(agentConfig)
	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start agent")

	// Wait for initial registration and heartbeat
	time.Sleep(1500 * time.Millisecond)

	// Verify agent is healthy
	agentInfo, err := server.GetAgent(agent.GetAgentID())
	helpers.AssertNoError(t, err, "Failed to get agent info")
	helpers.AssertEqual(t, "healthy", agentInfo.Status, "Agent should be healthy")

	t.Logf("Agent %s is healthy after heartbeats", agentInfo.ID)

	// Stop agent (no more heartbeats)
	agent.Stop()

	// Wait for heartbeat timeout (2s) + monitor check interval (5s) + buffer
	time.Sleep(8 * time.Second)

	// Verify agent is marked as unhealthy
	agentInfo, err = server.GetAgent(agent.GetAgentID())
	helpers.AssertNoError(t, err, "Failed to get agent info")
	helpers.AssertEqual(t, "unhealthy", agentInfo.Status, "Agent should be unhealthy after timeout")

	t.Logf("Agent %s marked as unhealthy after missing heartbeats", agentInfo.ID)
}

// TestServerTaskAssignment tests task assignment to agents
func TestServerTaskAssignment(t *testing.T) {
	server := mocks.NewDummyServer(nil)

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	// Start an agent
	agentConfig := &mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		HeartbeatInterval: 5 * time.Second,
	}

	agent := mocks.NewDummyAgent(agentConfig)
	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start agent")
	defer agent.Stop()

	// Wait for registration
	time.Sleep(500 * time.Millisecond)

	// Assign a task
	taskID := "test-task-1"
	url := "http://example.com/file.bin"

	err = server.AssignTask(agent.GetAgentID(), taskID, url)
	helpers.AssertNoError(t, err, "Failed to assign task")

	// Verify task is assigned
	task, err := server.GetTask(taskID)
	helpers.AssertNoError(t, err, "Failed to get task")

	helpers.AssertEqual(t, taskID, task.ID, "Task ID mismatch")
	helpers.AssertEqual(t, url, task.URL, "Task URL mismatch")
	helpers.AssertEqual(t, agent.GetAgentID(), task.AssignedTo, "Task assigned to wrong agent")
	helpers.AssertEqual(t, "assigned", task.Status, "Task status should be assigned")

	t.Logf("Task %s assigned to agent %s", taskID, agent.GetAgentID())
}

// TestServerProgressReporting tests progress update handling
func TestServerProgressReporting(t *testing.T) {
	server := mocks.NewDummyServer(nil)

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	// Start an agent
	agentConfig := &mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		HeartbeatInterval: 10 * time.Second,
	}

	agent := mocks.NewDummyAgent(agentConfig)
	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start agent")
	defer agent.Stop()

	// Wait for registration
	time.Sleep(500 * time.Millisecond)

	// Assign a task
	taskID := "progress-task-1"
	err = server.AssignTask(agent.GetAgentID(), taskID, "http://example.com/file.bin")
	helpers.AssertNoError(t, err, "Failed to assign task")

	// Report progress
	progressValues := []int32{10, 25, 50, 75, 100}

	for _, progress := range progressValues {
		// Use server helper since agent.ReportProgress is just a stub
		err = server.UpdateTaskProgress(taskID, progress)
		helpers.AssertNoError(t, err, "Failed to update progress")

		time.Sleep(100 * time.Millisecond)

		// Verify progress is updated
		task, err := server.GetTask(taskID)
		helpers.AssertNoError(t, err, "Failed to get task")
		helpers.AssertEqual(t, progress, task.Progress, "Progress mismatch")

		t.Logf("Task %s progress: %d%%", taskID, progress)
	}

	// Verify task is completed
	task, err := server.GetTask(taskID)
	helpers.AssertNoError(t, err, "Failed to get task")
	helpers.AssertEqual(t, "completed", task.Status, "Task should be completed")
}

// TestServerErrorReporting tests error reporting handling
func TestServerErrorReporting(t *testing.T) {
	server := mocks.NewDummyServer(nil)

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	// Start an agent
	agentConfig := &mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		HeartbeatInterval: 10 * time.Second,
	}

	agent := mocks.NewDummyAgent(agentConfig)
	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start agent")
	defer agent.Stop()

	// Wait for registration
	time.Sleep(500 * time.Millisecond)

	// Assign a task
	taskID := "error-task-1"
	err = server.AssignTask(agent.GetAgentID(), taskID, "http://example.com/missing.bin")
	helpers.AssertNoError(t, err, "Failed to assign task")

	// Report error
	errorMsg := "Download failed: 404 Not Found"
	err = server.UpdateTaskError(taskID, errorMsg)
	helpers.AssertNoError(t, err, "Failed to update error")

	time.Sleep(100 * time.Millisecond)

	// Verify error is recorded
	task, err := server.GetTask(taskID)
	helpers.AssertNoError(t, err, "Failed to get task")
	helpers.AssertEqual(t, "failed", task.Status, "Task should be failed")
	helpers.AssertEqual(t, errorMsg, task.Error, "Error message mismatch")

	t.Logf("Task %s failed with error: %s", taskID, task.Error)
}

// TestServerAgentReconnection tests agent reconnection handling
func TestServerAgentReconnection(t *testing.T) {
	server := mocks.NewDummyServer(nil)

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	// Start an agent
	agentConfig := &mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		AgentName:         "reconnect-agent-1",
		HeartbeatInterval: 2 * time.Second,
	}

	agent := mocks.NewDummyAgent(agentConfig)
	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start agent")

	// Wait for registration
	time.Sleep(500 * time.Millisecond)

	firstAgentID := agent.GetAgentID()
	t.Logf("Agent first registered with ID: %s", firstAgentID)

	// Stop agent
	agent.Stop()

	// Wait a bit
	time.Sleep(1 * time.Second)

	// Reconnect with same ID
	agent2 := mocks.NewDummyAgent(agentConfig)
	err = agent2.Start()
	helpers.AssertNoError(t, err, "Failed to reconnect agent")
	defer agent2.Stop()

	// Wait for registration
	time.Sleep(500 * time.Millisecond)

	secondAgentID := agent2.GetAgentID()
	t.Logf("Agent reconnected with ID: %s", secondAgentID)

	// For now, this creates a new registration
	// In a real implementation, you might want to preserve the agent ID
}

// TestServerBansErroneousAgentAndFailsOver verifies the server bans a repeatedly failing agent
// and completes the same task after reassigning it to a healthy agent.
func TestServerBansErroneousAgentAndFailsOver(t *testing.T) {
	server := mocks.NewDummyServer(&mocks.DummyServerConfig{
		Port:              0,
		HeartbeatTimeout:  30 * time.Second,
		AgentBanThreshold: 3,
	})

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	badAgent := mocks.NewDummyAgent(&mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		AgentName:         "bad-agent",
		HeartbeatInterval: 2 * time.Second,
		SimulateFailure:   true,
	})
	err = badAgent.Start()
	helpers.AssertNoError(t, err, "Failed to start bad agent")
	defer badAgent.Stop()

	goodAgent := mocks.NewDummyAgent(&mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		AgentName:         "good-agent",
		HeartbeatInterval: 2 * time.Second,
	})
	err = goodAgent.Start()
	helpers.AssertNoError(t, err, "Failed to start good agent")
	defer goodAgent.Stop()

	err = server.WaitForAgents(2, 5*time.Second)
	helpers.AssertNoError(t, err, "Expected both agents to register")

	taskID := "failover-task-1"
	err = server.AssignTask(badAgent.GetAgentID(), taskID, "http://example.com/file.bin")
	helpers.AssertNoError(t, err, "Failed to assign task to bad agent")

	for attempt := 1; attempt <= 3; attempt++ {
		err = badAgent.SimulateDownload(server, taskID, 300*time.Millisecond, 1.0)
		helpers.AssertNoError(t, err, "SimulateDownload call should complete while recording failure")

		task, getTaskErr := server.GetTask(taskID)
		helpers.AssertNoError(t, getTaskErr, "Failed to get task after failed attempt")
		helpers.AssertEqual(t, "failed", task.Status, "Task should be marked failed after bad agent attempt")
		t.Logf("Bad agent failure attempt %d recorded", attempt)
	}

	badInfo, err := server.GetAgent(badAgent.GetAgentID())
	helpers.AssertNoError(t, err, "Failed to get bad agent info")
	helpers.AssertEqual(t, "banned", badInfo.Status, "Bad agent should be banned after repeated failures")
	helpers.AssertEqual(t, 3, server.GetAgentFailureCount(badAgent.GetAgentID()), "Bad agent failure count mismatch")

	err = server.AssignTask(badAgent.GetAgentID(), "should-not-assign", "http://example.com/other.bin")
	if err == nil {
		t.Fatalf("expected assigning a task to banned agent to fail")
	}

	reassignedAgentID, err := server.ReassignTaskToAnyHealthyAgent(taskID)
	helpers.AssertNoError(t, err, "Failed to reassign task to healthy agent")
	helpers.AssertEqual(t, goodAgent.GetAgentID(), reassignedAgentID, "Task should be reassigned to good agent")

	err = goodAgent.SimulateDownload(server, taskID, 300*time.Millisecond, 1.0)
	helpers.AssertNoError(t, err, "Expected good agent to complete task")

	task, err := server.GetTask(taskID)
	helpers.AssertNoError(t, err, "Failed to read task after failover")
	helpers.AssertEqual(t, "completed", task.Status, "Task should complete after failover")
	helpers.AssertEqual(t, goodAgent.GetAgentID(), task.AssignedTo, "Task assignee should be good agent")

	t.Logf("Task %s completed by healthy agent %s after banning %s", taskID, goodAgent.GetAgentID(), badAgent.GetAgentID())
}
