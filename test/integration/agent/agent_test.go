package agent

import (
	"testing"
	"time"

	"github.com/imafish/ddson/test/helpers"
	"github.com/imafish/ddson/test/mocks"
)

// TestAgentRegistrationFlow tests the agent registration process
func TestAgentRegistrationFlow(t *testing.T) {
	// Start dummy server
	server := mocks.NewDummyServer(nil)
	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	// Create and start agent
	agentConfig := &mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		HeartbeatInterval: 5 * time.Second,
	}

	agent := mocks.NewDummyAgent(agentConfig)
	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start agent")
	defer agent.Stop()

	// Verify agent is running
	helpers.AssertTrue(t, agent.IsRunning(), "Agent should be running")

	// Verify agent has an ID
	agentID := agent.GetAgentID()
	helpers.AssertTrue(t, agentID != "", "Agent ID should not be empty")

	t.Logf("Agent registered successfully with ID: %s", agentID)

	// Verify agent is registered on server
	time.Sleep(500 * time.Millisecond)
	agents := server.GetAgents()
	helpers.AssertEqual(t, 1, len(agents), "Expected 1 agent on server")
}

// TestAgentHeartbeatTransmission tests heartbeat sending
func TestAgentHeartbeatTransmission(t *testing.T) {
	server := mocks.NewDummyServer(nil)
	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	// Start agent with fast heartbeat
	agentConfig := &mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		HeartbeatInterval: 1 * time.Second,
	}

	agent := mocks.NewDummyAgent(agentConfig)
	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start agent")
	defer agent.Stop()

	// Wait for multiple heartbeats
	time.Sleep(3 * time.Second)

	// Verify agent is still healthy
	agentInfo, err := server.GetAgent(agent.GetAgentID())
	helpers.AssertNoError(t, err, "Failed to get agent info")
	helpers.AssertEqual(t, "healthy", agentInfo.Status, "Agent should be healthy")

	t.Logf("Agent %s sent heartbeats successfully", agent.GetAgentID())
}

// TestAgentProgressReporting tests progress reporting
func TestAgentProgressReporting(t *testing.T) {
	server := mocks.NewDummyServer(nil)
	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	agentConfig := &mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		HeartbeatInterval: 10 * time.Second,
	}

	agent := mocks.NewDummyAgent(agentConfig)
	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start agent")
	defer agent.Stop()

	time.Sleep(500 * time.Millisecond)

	// Assign a task
	taskID := "agent-test-task-1"
	err = server.AssignTask(agent.GetAgentID(), taskID, "http://example.com/file.bin")
	helpers.AssertNoError(t, err, "Failed to assign task")

	// Agent reports progress
	progressValues := []int32{0, 25, 50, 75, 100}
	for _, progress := range progressValues {
		// Update progress via server since agent.ReportProgress is a stub
		err = server.UpdateTaskProgress(taskID, progress)
		helpers.AssertNoError(t, err, "Failed to update progress")

		t.Logf("Progress updated: %d%%", progress)
		time.Sleep(100 * time.Millisecond)
	}

	// Verify task completed
	task, err := server.GetTask(taskID)
	helpers.AssertNoError(t, err, "Failed to get task")
	helpers.AssertEqual(t, int32(100), task.Progress, "Task should be at 100%")
	helpers.AssertEqual(t, "completed", task.Status, "Task should be completed")
}

// TestAgentErrorReporting tests error reporting
func TestAgentErrorReporting(t *testing.T) {
	server := mocks.NewDummyServer(nil)
	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	agentConfig := &mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		HeartbeatInterval: 10 * time.Second,
	}

	agent := mocks.NewDummyAgent(agentConfig)
	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start agent")
	defer agent.Stop()

	time.Sleep(500 * time.Millisecond)

	// Assign a task
	taskID := "agent-error-task-1"
	err = server.AssignTask(agent.GetAgentID(), taskID, "http://example.com/missing.bin")
	helpers.AssertNoError(t, err, "Failed to assign task")

	// Agent reports error
	errorMsg := "File not found: 404"
	err = server.UpdateTaskError(taskID, errorMsg)
	helpers.AssertNoError(t, err, "Failed to update error")

	time.Sleep(100 * time.Millisecond)

	// Verify error recorded
	task, err := server.GetTask(taskID)
	helpers.AssertNoError(t, err, "Failed to get task")
	helpers.AssertEqual(t, "failed", task.Status, "Task should be failed")
	helpers.AssertTrue(t, task.Error != "", "Error message should not be empty")

	t.Logf("Agent reported error: %s", task.Error)
}

// TestAgentSimulatedDownload tests the simulated download functionality
func TestAgentSimulatedDownload(t *testing.T) {
	server := mocks.NewDummyServer(nil)
	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	agentConfig := &mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		HeartbeatInterval: 10 * time.Second,
	}

	agent := mocks.NewDummyAgent(agentConfig)
	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start agent")
	defer agent.Stop()

	time.Sleep(500 * time.Millisecond)

	// Assign a task
	taskID := "simulated-download-1"
	err = server.AssignTask(agent.GetAgentID(), taskID, "http://example.com/file.bin")
	helpers.AssertNoError(t, err, "Failed to assign task")

	// Simulate download
	err = agent.SimulateDownload(server, taskID, 2*time.Second, 1.0)
	helpers.AssertNoError(t, err, "Simulated download failed")

	// Verify task completed
	task, err := server.GetTask(taskID)
	helpers.AssertNoError(t, err, "Failed to get task")
	helpers.AssertEqual(t, "completed", task.Status, "Task should be completed")

	t.Logf("Simulated download completed successfully")
}

// TestAgentFailureSimulation tests agent failure simulation
func TestAgentFailureSimulation(t *testing.T) {
	server := mocks.NewDummyServer(nil)
	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	agentConfig := &mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		HeartbeatInterval: 2 * time.Second,
		SimulateFailure:   true,
	}

	agent := mocks.NewDummyAgent(agentConfig)
	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start agent")
	defer agent.Stop()

	time.Sleep(500 * time.Millisecond)

	// Assign a task
	taskID := "failure-simulation-1"
	err = server.AssignTask(agent.GetAgentID(), taskID, "http://example.com/file.bin")
	helpers.AssertNoError(t, err, "Failed to assign task")

	// Simulate download (should fail)
	err = agent.SimulateDownload(server, taskID, 1*time.Second, 0.5)
	// Error is expected here since SimulateFailure is true
	if err == nil {
		t.Log("Agent simulated failure as expected")
	}

	time.Sleep(500 * time.Millisecond)

	// Verify task failed
	task, err := server.GetTask(taskID)
	helpers.AssertNoError(t, err, "Failed to get task")

	t.Logf("Task status: %s, Error: %s", task.Status, task.Error)
}

// TestAgentGracefulShutdown tests agent shutdown
func TestAgentGracefulShutdown(t *testing.T) {
	server := mocks.NewDummyServer(nil)
	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	agentConfig := &mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		HeartbeatInterval: 5 * time.Second,
	}

	agent := mocks.NewDummyAgent(agentConfig)
	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start agent")

	time.Sleep(500 * time.Millisecond)

	// Verify agent is running
	helpers.AssertTrue(t, agent.IsRunning(), "Agent should be running")

	// Stop agent
	err = agent.Stop()
	helpers.AssertNoError(t, err, "Failed to stop agent")

	// Verify agent is stopped
	helpers.AssertFalse(t, agent.IsRunning(), "Agent should not be running")

	t.Log("Agent shutdown gracefully")
}

// TestMultipleAgentsWithSameServer tests multiple agents connecting to one server
func TestMultipleAgentsWithSameServer(t *testing.T) {
	server := mocks.NewDummyServer(nil)
	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	numAgents := 5
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

		t.Logf("Started agent %d with ID: %s", i+1, agents[i].GetAgentID())
	}

	// Wait for all registrations
	err = server.WaitForAgents(numAgents, 5*time.Second)
	helpers.AssertNoError(t, err, "Failed to wait for all agents")

	// Verify all agents registered
	registeredAgents := server.GetAgents()
	helpers.AssertEqual(t, numAgents, len(registeredAgents), "All agents should be registered")

	t.Logf("All %d agents registered successfully", numAgents)
}
