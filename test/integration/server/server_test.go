package server

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

// getServerExecutablePath returns the path to the test server executable
func getServerExecutablePath() string {
	// Get the directory of this source file
	_, filename, _, _ := runtime.Caller(0)
	sourceDir := filepath.Dir(filename)
	// Navigate to project root and find the executable
	projectRoot, _ := filepath.Abs(filepath.Join(sourceDir, "..", "..", ".."))
	execPath := filepath.Join(projectRoot, "test", "bin", "ddson_server")
	// Check if it exists, if not try output directory
	if _, err := os.Stat(execPath); os.IsNotExist(err) {
		execPath = filepath.Join(projectRoot, "output", "ddson_server_linux_amd64")
	}
	return execPath
}

// TestExecutableServerAgentRegistration tests agent registration with real server executable
func TestExecutableServerAgentRegistration(t *testing.T) {
	t.Parallel()

	// Start real server executable
	server := mocks.NewExecutableServer(&mocks.ExecutableServerConfig{
		ExecPath: getServerExecutablePath(),
		Port:     0,
	})

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start executable server")
	defer server.Stop()

	t.Logf("Executable server started at: %s", server.Addr())

	// Start dummy agent
	agent := mocks.NewDummyAgent(&mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		AgentName:         "test-agent-1",
		HeartbeatInterval: 5 * time.Second,
	})

	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy agent")
	defer agent.Stop()

	// Wait for registration
	time.Sleep(2 * time.Second)

	// Verify agent is running
	helpers.AssertTrue(t, agent.IsRunning(), "Agent should be running")
	helpers.AssertTrue(t, agent.GetAgentID() != "", "Agent should have an ID")

	t.Logf("Agent registered with ID: %s", agent.GetAgentID())
}

// TestExecutableServerMultipleAgents tests multiple agents registering with real server
func TestExecutableServerMultipleAgents(t *testing.T) {
	t.Parallel()

	// Start real server executable
	server := mocks.NewExecutableServer(&mocks.ExecutableServerConfig{
		ExecPath: getServerExecutablePath(),
		Port:     0,
	})

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start executable server")
	defer server.Stop()

	t.Logf("Executable server started at: %s", server.Addr())

	// Start multiple dummy agents
	numAgents := 3
	agents := make([]*mocks.DummyAgent, numAgents)

	for i := 0; i < numAgents; i++ {
		agents[i] = mocks.NewDummyAgent(&mocks.DummyAgentConfig{
			ServerAddr:        server.Addr(),
			AgentName:         fmt.Sprintf("test-agent-%d", i+1),
			Port:              int32(60000 + i), // Unique port for each agent
			HeartbeatInterval: 5 * time.Second,
		})

		err := agents[i].Start()
		helpers.AssertNoError(t, err, fmt.Sprintf("Failed to start agent %d", i+1))
		defer agents[i].Stop()
	}

	// Wait for all agents to register
	time.Sleep(3 * time.Second)

	// Verify all agents are running
	for i, agent := range agents {
		helpers.AssertTrue(t, agent.IsRunning(), fmt.Sprintf("Agent %d should be running", i+1))
		t.Logf("Agent %d registered with ID: %s", i+1, agent.GetAgentID())
	}
}

// TestExecutableServerHeartbeatHandling tests heartbeat handling with real server
func TestExecutableServerHeartbeatHandling(t *testing.T) {
	t.Parallel()

	// Start real server executable
	server := mocks.NewExecutableServer(&mocks.ExecutableServerConfig{
		ExecPath: getServerExecutablePath(),
		Port:     0,
	})

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start executable server")
	defer server.Stop()

	t.Logf("Executable server started at: %s", server.Addr())

	// Start dummy agent with short heartbeat interval
	agent := mocks.NewDummyAgent(&mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		AgentName:         "heartbeat-agent",
		HeartbeatInterval: 1 * time.Second,
	})

	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start dummy agent")
	defer agent.Stop()

	// Wait for initial registration and several heartbeats
	time.Sleep(5 * time.Second)

	// Agent should still be running
	helpers.AssertTrue(t, agent.IsRunning(), "Agent should still be running")
	t.Logf("Agent %s sent heartbeats successfully", agent.GetAgentID())
}

// TestExecutableServerAgentReconnection tests agent reconnection with real server
func TestExecutableServerAgentReconnection(t *testing.T) {
	t.Parallel()

	// Start real server executable
	server := mocks.NewExecutableServer(&mocks.ExecutableServerConfig{
		ExecPath: getServerExecutablePath(),
		Port:     0,
	})

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start executable server")
	defer server.Stop()

	t.Logf("Executable server started at: %s", server.Addr())

	// Start agent
	agent1 := mocks.NewDummyAgent(&mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		AgentName:         "reconnect-agent",
		Port:              60010,
		HeartbeatInterval: 2 * time.Second,
	})

	err = agent1.Start()
	helpers.AssertNoError(t, err, "Failed to start agent")

	time.Sleep(2 * time.Second)
	firstID := agent1.GetAgentID()
	t.Logf("Agent first connected with ID: %s", firstID)

	// Stop agent
	agent1.Stop()
	time.Sleep(1 * time.Second)

	// Reconnect
	agent2 := mocks.NewDummyAgent(&mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		AgentName:         "reconnect-agent",
		Port:              60011, // Different port for reconnection
		HeartbeatInterval: 2 * time.Second,
	})

	err = agent2.Start()
	helpers.AssertNoError(t, err, "Failed to reconnect agent")
	defer agent2.Stop()

	time.Sleep(2 * time.Second)
	secondID := agent2.GetAgentID()
	t.Logf("Agent reconnected with ID: %s", secondID)

	helpers.AssertTrue(t, agent2.IsRunning(), "Reconnected agent should be running")
}

// TestExecutableServerWithDownloadServer tests server with download server integration
func TestExecutableServerWithDownloadServer(t *testing.T) {
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

	// Start real server executable
	server := mocks.NewExecutableServer(&mocks.ExecutableServerConfig{
		ExecPath: getServerExecutablePath(),
		Port:     0,
	})

	err = server.Start()
	helpers.AssertNoError(t, err, "Failed to start executable server")
	defer server.Stop()

	t.Logf("Executable server started at: %s", server.Addr())

	// Start dummy agent
	agent := mocks.NewDummyAgent(&mocks.DummyAgentConfig{
		ServerAddr:        server.Addr(),
		AgentName:         "download-agent",
		HeartbeatInterval: 5 * time.Second,
	})

	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start agent")
	defer agent.Stop()

	// Wait for registration
	time.Sleep(2 * time.Second)

	helpers.AssertTrue(t, agent.IsRunning(), "Agent should be running")
	t.Logf("Setup complete - Server, Agent, and Download Server all running")
}

// TestExecutableServerStopsCleanly tests that the server stops cleanly
func TestExecutableServerStopsCleanly(t *testing.T) {
	t.Parallel()

	server := mocks.NewExecutableServer(&mocks.ExecutableServerConfig{
		ExecPath: getServerExecutablePath(),
		Port:     0,
	})

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start server")

	helpers.AssertTrue(t, server.IsRunning(), "Server should be running")
	t.Logf("Server started at: %s", server.Addr())

	// Stop server
	err = server.Stop()
	helpers.AssertNoError(t, err, "Failed to stop server")

	helpers.AssertTrue(t, !server.IsRunning(), "Server should not be running after stop")
	t.Log("Server stopped cleanly")
}

// TestExecutableServerWorkspaceIsolation tests that each server has isolated workspace
func TestExecutableServerWorkspaceIsolation(t *testing.T) {
	t.Parallel()

	// Start two servers
	server1 := mocks.NewExecutableServer(&mocks.ExecutableServerConfig{
		ExecPath: getServerExecutablePath(),
		Port:     0,
	})

	err := server1.Start()
	helpers.AssertNoError(t, err, "Failed to start server 1")
	defer server1.Stop()

	server2 := mocks.NewExecutableServer(&mocks.ExecutableServerConfig{
		ExecPath: getServerExecutablePath(),
		Port:     0,
	})

	err = server2.Start()
	helpers.AssertNoError(t, err, "Failed to start server 2")
	defer server2.Stop()

	// Verify different addresses
	helpers.AssertTrue(t, server1.Addr() != server2.Addr(), "Servers should have different addresses")

	// Verify different workspaces
	helpers.AssertTrue(t, server1.GetWorkspaceDir() != server2.GetWorkspaceDir(), "Servers should have different workspaces")

	t.Logf("Server 1: %s (workspace: %s)", server1.Addr(), server1.GetWorkspaceDir())
	t.Logf("Server 2: %s (workspace: %s)", server2.Addr(), server2.GetWorkspaceDir())
}
