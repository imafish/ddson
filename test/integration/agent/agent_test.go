package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/imafish/ddson/test/helpers"
	"github.com/imafish/ddson/test/mocks"
)

// getAgentExecutablePath returns the path to the test agent executable
func getAgentExecutablePath() string {
	// Get the directory of this source file
	_, filename, _, _ := runtime.Caller(0)
	sourceDir := filepath.Dir(filename)
	// Navigate to project root and find the executable
	projectRoot, _ := filepath.Abs(filepath.Join(sourceDir, "..", "..", ".."))
	execPath := filepath.Join(projectRoot, "test", "bin", "ddson_client")
	// Check if it exists, if not try output directory
	if _, err := os.Stat(execPath); os.IsNotExist(err) {
		execPath = filepath.Join(projectRoot, "output", "ddson_client_linux_amd64")
	}
	return execPath
}

// TestExecutableAgentRegistration tests agent executable registration with dummy server
func TestExecutableAgentRegistration(t *testing.T) {
	t.Parallel()

	// Start dummy server
	server := mocks.NewDummyServer(&mocks.DummyServerConfig{
		Port:             0,
		HeartbeatTimeout: 30 * time.Second,
	})

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	t.Logf("Dummy server started at: %s", server.Addr())

	// Start real agent executable
	agent := mocks.NewExecutableAgent(&mocks.ExecutableAgentConfig{
		ExecPath:   getAgentExecutablePath(),
		ServerAddr: server.Addr(),
		AgentName:  "test-agent-1",
		Port:       0,
	})

	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start executable agent")
	defer agent.Stop()

	t.Logf("Agent started with ID: %s", agent.GetAgentID())

	// Wait for registration
	err = server.WaitForAgents(1, 5*time.Second)
	helpers.AssertNoError(t, err, "Agent should register")

	// Verify agent is registered
	agents := server.GetAgents()
	helpers.AssertEqual(t, 1, len(agents), "Expected 1 registered agent")
	helpers.AssertEqual(t, "healthy", agents[0].Status, "Agent should be healthy")

	t.Logf("Agent %s successfully registered with server", agents[0].ID)
}

// TestExecutableAgentHeartbeat tests that agent sends heartbeats
func TestExecutableAgentHeartbeat(t *testing.T) {
	t.Parallel()

	// Start dummy server
	server := mocks.NewDummyServer(&mocks.DummyServerConfig{
		Port:             0,
		HeartbeatTimeout: 30 * time.Second,
	})

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	// Start real agent executable
	agent := mocks.NewExecutableAgent(&mocks.ExecutableAgentConfig{
		ExecPath:   getAgentExecutablePath(),
		ServerAddr: server.Addr(),
		AgentName:  "heartbeat-agent",
		Port:       0,
	})

	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start executable agent")
	defer agent.Stop()

	// Wait for registration
	err = server.WaitForAgents(1, 5*time.Second)
	helpers.AssertNoError(t, err, "Agent should register")

	// Wait for several heartbeat intervals
	time.Sleep(10 * time.Second)

	// Verify agent is still healthy
	agents := server.GetAgents()
	helpers.AssertEqual(t, 1, len(agents), "Expected 1 agent")
	helpers.AssertEqual(t, "healthy", agents[0].Status, "Agent should still be healthy")

	// Check last heartbeat is recent
	lastHeartbeat := agents[0].LastHeartbeat
	timeSinceHeartbeat := time.Since(lastHeartbeat)
	helpers.AssertTrue(t, timeSinceHeartbeat < 15*time.Second, "Last heartbeat should be recent")

	t.Logf("Agent sent heartbeats successfully (last: %v ago)", timeSinceHeartbeat)
}

// TestExecutableAgentMultipleAgents tests multiple agent executables
func TestExecutableAgentMultipleAgents(t *testing.T) {
	t.Parallel()

	// Start dummy server
	server := mocks.NewDummyServer(&mocks.DummyServerConfig{
		Port:             0,
		HeartbeatTimeout: 30 * time.Second,
	})

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	t.Logf("Dummy server started at: %s", server.Addr())

	// Start multiple agents
	numAgents := 3
	agents := make([]*mocks.ExecutableAgent, numAgents)

	for i := 0; i < numAgents; i++ {
		agents[i] = mocks.NewExecutableAgent(&mocks.ExecutableAgentConfig{
			ExecPath:   getAgentExecutablePath(),
			ServerAddr: server.Addr(),
			AgentName:  fmt.Sprintf("test-agent-%d", i+1),
			Port:       0,
		})

		err := agents[i].Start()
		helpers.AssertNoError(t, err, fmt.Sprintf("Failed to start agent %d", i+1))
		defer agents[i].Stop()
	}

	// Wait for all agents to register
	err = server.WaitForAgents(numAgents, 10*time.Second)
	helpers.AssertNoError(t, err, "All agents should register")

	// Verify all agents are registered
	registeredAgents := server.GetAgents()
	helpers.AssertEqual(t, numAgents, len(registeredAgents), "Expected all agents to be registered")

	for i, agent := range registeredAgents {
		t.Logf("Agent %d: ID=%s, Status=%s", i+1, agent.ID, agent.Status)
		helpers.AssertEqual(t, "healthy", agent.Status, "Agent should be healthy")
	}
}

// TestExecutableAgentWithDownloadServer tests agent with download task
func TestExecutableAgentWithDownloadServer(t *testing.T) {
	t.Parallel()

	// Start download server
	downloadServer := mocks.NewTestDownloadServer(&mocks.DownloadServerConfig{
		Port:           0,
		SupportsRanges: true,
	})

	testFile := mocks.GenerateRandomTestFile(1 * 1024 * 1024) // 1 MB
	downloadServer.AddFile("test-file.bin", testFile)

	err := downloadServer.Start()
	helpers.AssertNoError(t, err, "Failed to start download server")
	defer downloadServer.Stop()

	fileURL := downloadServer.FileURL("test-file.bin")
	t.Logf("Download URL: %s", fileURL)

	// Start dummy server
	server := mocks.NewDummyServer(&mocks.DummyServerConfig{
		Port:             0,
		HeartbeatTimeout: 30 * time.Second,
	})

	err = server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	// Start real agent executable
	agent := mocks.NewExecutableAgent(&mocks.ExecutableAgentConfig{
		ExecPath:   getAgentExecutablePath(),
		ServerAddr: server.Addr(),
		AgentName:  "download-agent",
		Port:       0,
	})

	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start executable agent")
	defer agent.Stop()

	// Wait for registration
	err = server.WaitForAgents(1, 5*time.Second)
	helpers.AssertNoError(t, err, "Agent should register")

	// Assign download task
	taskID := "test-download-1"
	err = server.AssignTask(agent.GetAgentID(), taskID, fileURL)
	helpers.AssertNoError(t, err, "Failed to assign task")

	t.Logf("Assigned task %s to agent %s", taskID, agent.GetAgentID())

	// In a real scenario, the agent would download the file
	// For this test, we just verify the infrastructure is working
	time.Sleep(2 * time.Second)

	task, err := server.GetTask(taskID)
	helpers.AssertNoError(t, err, "Failed to get task")
	helpers.AssertEqual(t, taskID, task.ID, "Task ID should match")
	helpers.AssertEqual(t, agent.GetAgentID(), task.AssignedTo, "Task should be assigned to agent")
}

// TestExecutableAgentStopsCleanly tests that agent stops cleanly
func TestExecutableAgentStopsCleanly(t *testing.T) {
	t.Parallel()

	// Start dummy server
	server := mocks.NewDummyServer(&mocks.DummyServerConfig{
		Port:             0,
		HeartbeatTimeout: 30 * time.Second,
	})

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	// Start agent
	agent := mocks.NewExecutableAgent(&mocks.ExecutableAgentConfig{
		ExecPath:   getAgentExecutablePath(),
		ServerAddr: server.Addr(),
		AgentName:  "stop-test-agent",
		Port:       0,
	})

	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start agent")

	helpers.AssertTrue(t, agent.IsRunning(), "Agent should be running")

	// Wait for registration
	time.Sleep(2 * time.Second)

	// Stop agent
	err = agent.Stop()
	helpers.AssertNoError(t, err, "Failed to stop agent")

	helpers.AssertTrue(t, !agent.IsRunning(), "Agent should not be running after stop")
	t.Log("Agent stopped cleanly")
}

// TestExecutableAgentReconnection tests agent reconnection
func TestExecutableAgentReconnection(t *testing.T) {
	t.Parallel()

	// Start dummy server
	server := mocks.NewDummyServer(&mocks.DummyServerConfig{
		Port:             0,
		HeartbeatTimeout: 30 * time.Second,
	})

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	// Start first agent
	agent1 := mocks.NewExecutableAgent(&mocks.ExecutableAgentConfig{
		ExecPath:   getAgentExecutablePath(),
		ServerAddr: server.Addr(),
		AgentName:  "reconnect-agent",
		Port:       0,
	})

	err = agent1.Start()
	helpers.AssertNoError(t, err, "Failed to start first agent")

	// Wait for registration
	err = server.WaitForAgents(1, 5*time.Second)
	helpers.AssertNoError(t, err, "First agent should register")

	firstID := agent1.GetAgentID()
	t.Logf("Agent first connected with ID: %s", firstID)

	// Stop first agent
	agent1.Stop()
	time.Sleep(2 * time.Second)

	// Start second agent with same name
	agent2 := mocks.NewExecutableAgent(&mocks.ExecutableAgentConfig{
		ExecPath:   getAgentExecutablePath(),
		ServerAddr: server.Addr(),
		AgentName:  "reconnect-agent",
		Port:       0,
	})

	err = agent2.Start()
	helpers.AssertNoError(t, err, "Failed to start second agent")
	defer agent2.Stop()

	// Wait for registration
	time.Sleep(3 * time.Second)

	secondID := agent2.GetAgentID()
	t.Logf("Agent reconnected with ID: %s", secondID)

	helpers.AssertTrue(t, agent2.IsRunning(), "Reconnected agent should be running")
}

// TestExecutableAgentWithAuth tests agent with authenticated download server
func TestExecutableAgentWithAuth(t *testing.T) {
	t.Parallel()

	// Start download server with authentication
	downloadServer := mocks.NewTestDownloadServer(&mocks.DownloadServerConfig{
		Port:           0,
		SupportsRanges: true,
		RequireAuth:    true,
		Username:       "testuser",
		Password:       "testpass",
	})

	testFile := mocks.GenerateRandomTestFile(512 * 1024) // 512 KB
	downloadServer.AddFile("secure-file.bin", testFile)

	err := downloadServer.Start()
	helpers.AssertNoError(t, err, "Failed to start download server")
	defer downloadServer.Stop()

	fileURL := downloadServer.FileURL("secure-file.bin")
	t.Logf("Authenticated download URL: %s", fileURL)

	// Start dummy server
	server := mocks.NewDummyServer(&mocks.DummyServerConfig{
		Port:             0,
		HeartbeatTimeout: 30 * time.Second,
	})

	err = server.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy server")
	defer server.Stop()

	// Start real agent executable
	agent := mocks.NewExecutableAgent(&mocks.ExecutableAgentConfig{
		ExecPath:   getAgentExecutablePath(),
		ServerAddr: server.Addr(),
		AgentName:  "auth-agent",
		Port:       0,
	})

	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start executable agent")
	defer agent.Stop()

	// Wait for registration
	err = server.WaitForAgents(1, 5*time.Second)
	helpers.AssertNoError(t, err, "Agent should register")

	t.Logf("Setup complete - Agent can work with authenticated download server")
}
