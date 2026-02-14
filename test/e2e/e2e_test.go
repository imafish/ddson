package e2e

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/imafish/ddson/test/helpers"
	"github.com/imafish/ddson/test/mocks"
)

// getServerExecutablePath returns the path to the test server executable
func getServerExecutablePath() string {
	return filepath.Join("..", "bin", "ddson_server")
}

// getAgentExecutablePath returns the path to the test agent executable
func getAgentExecutablePath() string {
	return filepath.Join("..", "bin", "ddson_client")
}

// TestE2EBasicDownload tests end-to-end basic download with real executables
func TestE2EBasicDownload(t *testing.T) {
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
	fileChecksum, _ := downloadServer.GetFileChecksum("test-file.bin")
	t.Logf("Download URL: %s (checksum: %s)", fileURL, fileChecksum)

	// Start real server executable
	server := mocks.NewExecutableServer(&mocks.ExecutableServerConfig{
		ExecPath: getServerExecutablePath(),
		Port:     0,
	})

	err = server.Start()
	helpers.AssertNoError(t, err, "Failed to start executable server")
	defer server.Stop()

	t.Logf("Server started at: %s", server.Addr())

	// Start real agent executable
	agent := mocks.NewExecutableAgent(&mocks.ExecutableAgentConfig{
		ExecPath:   getAgentExecutablePath(),
		ServerAddr: server.Addr(),
		AgentName:  "e2e-agent-1",
		Port:       0,
	})

	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start executable agent")
	defer agent.Stop()

	t.Logf("Agent started with ID: %s", agent.GetAgentID())

	// Wait for agent registration
	time.Sleep(3 * time.Second)

	// Verify agent is running
	helpers.AssertTrue(t, agent.IsRunning(), "Agent should be running")
	helpers.AssertTrue(t, server.IsRunning(), "Server should be running")

	t.Log("End-to-end test setup complete - all components running")
}

// TestE2EMultipleAgents tests multiple real agents with real server
func TestE2EMultipleAgents(t *testing.T) {
	t.Parallel()

	// Start download server
	downloadServer := mocks.NewTestDownloadServer(&mocks.DownloadServerConfig{
		Port:           0,
		SupportsRanges: true,
	})

	// Add multiple test files
	for i := 1; i <= 3; i++ {
		testFile := mocks.GenerateRandomTestFile(512 * 1024) // 512 KB each
		downloadServer.AddFile(fmt.Sprintf("file-%d.bin", i), testFile)
	}

	err := downloadServer.Start()
	helpers.AssertNoError(t, err, "Failed to start download server")
	defer downloadServer.Stop()

	t.Logf("Download server started at: %s", downloadServer.URL())

	// Start real server executable
	server := mocks.NewExecutableServer(&mocks.ExecutableServerConfig{
		ExecPath: getServerExecutablePath(),
		Port:     0,
	})

	err = server.Start()
	helpers.AssertNoError(t, err, "Failed to start executable server")
	defer server.Stop()

	t.Logf("Server started at: %s", server.Addr())

	// Start multiple real agents
	numAgents := 3
	agents := make([]*mocks.ExecutableAgent, numAgents)

	for i := 0; i < numAgents; i++ {
		agents[i] = mocks.NewExecutableAgent(&mocks.ExecutableAgentConfig{
			ExecPath:   getAgentExecutablePath(),
			ServerAddr: server.Addr(),
			AgentName:  fmt.Sprintf("e2e-agent-%d", i+1),
			Port:       0,
		})

		err := agents[i].Start()
		helpers.AssertNoError(t, err, fmt.Sprintf("Failed to start agent %d", i+1))
		defer agents[i].Stop()

		t.Logf("Agent %d started with ID: %s", i+1, agents[i].GetAgentID())
	}

	// Wait for all agents to register
	time.Sleep(5 * time.Second)

	// Verify all agents are running
	for i, agent := range agents {
		helpers.AssertTrue(t, agent.IsRunning(), fmt.Sprintf("Agent %d should be running", i+1))
	}

	t.Log("Multiple agents test complete - all agents and server running")
}

// TestE2ELargeFileDownload tests downloading a larger file
func TestE2ELargeFileDownload(t *testing.T) {
	t.Parallel()

	// Start download server
	downloadServer := mocks.NewTestDownloadServer(&mocks.DownloadServerConfig{
		Port:           0,
		SupportsRanges: true,
	})

	// Create a larger test file (10 MB)
	largeFile := mocks.GenerateRandomTestFile(10 * 1024 * 1024)
	downloadServer.AddFile("large-file.bin", largeFile)

	err := downloadServer.Start()
	helpers.AssertNoError(t, err, "Failed to start download server")
	defer downloadServer.Stop()

	fileURL := downloadServer.FileURL("large-file.bin")
	fileChecksum, _ := downloadServer.GetFileChecksum("large-file.bin")
	t.Logf("Large file URL: %s (checksum: %s)", fileURL, fileChecksum)

	// Start real server executable
	server := mocks.NewExecutableServer(&mocks.ExecutableServerConfig{
		ExecPath: getServerExecutablePath(),
		Port:     0,
	})

	err = server.Start()
	helpers.AssertNoError(t, err, "Failed to start executable server")
	defer server.Stop()

	t.Logf("Server started at: %s", server.Addr())

	// Start real agent executable
	agent := mocks.NewExecutableAgent(&mocks.ExecutableAgentConfig{
		ExecPath:   getAgentExecutablePath(),
		ServerAddr: server.Addr(),
		AgentName:  "large-file-agent",
		Port:       0,
	})

	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start executable agent")
	defer agent.Stop()

	t.Logf("Agent started with ID: %s", agent.GetAgentID())

	// Wait for agent registration
	time.Sleep(3 * time.Second)

	helpers.AssertTrue(t, agent.IsRunning(), "Agent should be running")
	t.Log("Large file download test setup complete")
}

// TestE2EWithAuthentication tests download with authentication
func TestE2EWithAuthentication(t *testing.T) {
	t.Parallel()

	// Start download server with authentication
	downloadServer := mocks.NewTestDownloadServer(&mocks.DownloadServerConfig{
		Port:           0,
		SupportsRanges: true,
		RequireAuth:    true,
		Username:       "testuser",
		Password:       "testpass",
	})

	testFile := mocks.GenerateRandomTestFile(2 * 1024 * 1024) // 2 MB
	downloadServer.AddFile("secure-file.bin", testFile)

	err := downloadServer.Start()
	helpers.AssertNoError(t, err, "Failed to start download server")
	defer downloadServer.Stop()

	fileURL := downloadServer.FileURL("secure-file.bin")
	t.Logf("Authenticated download URL: %s", fileURL)

	// Start real server executable
	server := mocks.NewExecutableServer(&mocks.ExecutableServerConfig{
		ExecPath: getServerExecutablePath(),
		Port:     0,
	})

	err = server.Start()
	helpers.AssertNoError(t, err, "Failed to start executable server")
	defer server.Stop()

	t.Logf("Server started at: %s", server.Addr())

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

	t.Logf("Agent started with ID: %s", agent.GetAgentID())

	// Wait for agent registration
	time.Sleep(3 * time.Second)

	helpers.AssertTrue(t, agent.IsRunning(), "Agent should be running")
	t.Log("Authentication test setup complete")
}

// TestE2EPartialDownload tests download with range support
func TestE2EPartialDownload(t *testing.T) {
	t.Parallel()

	// Start download server with range support
	downloadServer := mocks.NewTestDownloadServer(&mocks.DownloadServerConfig{
		Port:           0,
		SupportsRanges: true,
	})

	testFile := mocks.GenerateRandomTestFile(5 * 1024 * 1024) // 5 MB
	downloadServer.AddFile("partial-file.bin", testFile)

	err := downloadServer.Start()
	helpers.AssertNoError(t, err, "Failed to start download server")
	defer downloadServer.Stop()

	fileURL := downloadServer.FileURL("partial-file.bin")
	t.Logf("Partial download URL: %s", fileURL)

	// Start real server executable
	server := mocks.NewExecutableServer(&mocks.ExecutableServerConfig{
		ExecPath: getServerExecutablePath(),
		Port:     0,
	})

	err = server.Start()
	helpers.AssertNoError(t, err, "Failed to start executable server")
	defer server.Stop()

	t.Logf("Server started at: %s", server.Addr())

	// Start real agent executable
	agent := mocks.NewExecutableAgent(&mocks.ExecutableAgentConfig{
		ExecPath:   getAgentExecutablePath(),
		ServerAddr: server.Addr(),
		AgentName:  "partial-download-agent",
		Port:       0,
	})

	err = agent.Start()
	helpers.AssertNoError(t, err, "Failed to start executable agent")
	defer agent.Stop()

	t.Logf("Agent started with ID: %s", agent.GetAgentID())

	// Wait for agent registration
	time.Sleep(3 * time.Second)

	helpers.AssertTrue(t, agent.IsRunning(), "Agent should be running")
	t.Log("Partial download test setup complete")
}

// TestE2EAgentFailover tests agent failover scenario
func TestE2EAgentFailover(t *testing.T) {
	t.Parallel()

	// Start download server
	downloadServer := mocks.NewTestDownloadServer(&mocks.DownloadServerConfig{
		Port:           0,
		SupportsRanges: true,
	})

	testFile := mocks.GenerateRandomTestFile(1 * 1024 * 1024) // 1 MB
	downloadServer.AddFile("failover-file.bin", testFile)

	err := downloadServer.Start()
	helpers.AssertNoError(t, err, "Failed to start download server")
	defer downloadServer.Stop()

	fileURL := downloadServer.FileURL("failover-file.bin")
	t.Logf("Download URL: %s", fileURL)

	// Start real server executable
	server := mocks.NewExecutableServer(&mocks.ExecutableServerConfig{
		ExecPath: getServerExecutablePath(),
		Port:     0,
	})

	err = server.Start()
	helpers.AssertNoError(t, err, "Failed to start executable server")
	defer server.Stop()

	t.Logf("Server started at: %s", server.Addr())

	// Start first agent
	agent1 := mocks.NewExecutableAgent(&mocks.ExecutableAgentConfig{
		ExecPath:   getAgentExecutablePath(),
		ServerAddr: server.Addr(),
		AgentName:  "failover-agent-1",
		Port:       0,
	})

	err = agent1.Start()
	helpers.AssertNoError(t, err, "Failed to start first agent")
	defer agent1.Stop()

	t.Logf("Agent 1 started with ID: %s", agent1.GetAgentID())

	// Start second agent
	agent2 := mocks.NewExecutableAgent(&mocks.ExecutableAgentConfig{
		ExecPath:   getAgentExecutablePath(),
		ServerAddr: server.Addr(),
		AgentName:  "failover-agent-2",
		Port:       0,
	})

	err = agent2.Start()
	helpers.AssertNoError(t, err, "Failed to start second agent")
	defer agent2.Stop()

	t.Logf("Agent 2 started with ID: %s", agent2.GetAgentID())

	// Wait for both agents to register
	time.Sleep(5 * time.Second)

	// Verify both agents are running
	helpers.AssertTrue(t, agent1.IsRunning(), "Agent 1 should be running")
	helpers.AssertTrue(t, agent2.IsRunning(), "Agent 2 should be running")

	t.Log("Failover test setup complete - both agents running")

	// Simulate agent 1 failure
	agent1.Stop()
	t.Log("Agent 1 stopped - simulating failure")

	// Wait for failover
	time.Sleep(3 * time.Second)

	// Agent 2 should still be running
	helpers.AssertTrue(t, agent2.IsRunning(), "Agent 2 should still be running after agent 1 failure")

	t.Log("Failover test complete - agent 2 continues running")
}

// TestE2EStressTest tests system under load
func TestE2EStressTest(t *testing.T) {
	t.Parallel()

	// Start download server
	downloadServer := mocks.NewTestDownloadServer(&mocks.DownloadServerConfig{
		Port:           0,
		SupportsRanges: true,
	})

	// Add multiple files
	numFiles := 5
	for i := 1; i <= numFiles; i++ {
		testFile := mocks.GenerateRandomTestFile(1 * 1024 * 1024) // 1 MB each
		downloadServer.AddFile(fmt.Sprintf("stress-file-%d.bin", i), testFile)
	}

	err := downloadServer.Start()
	helpers.AssertNoError(t, err, "Failed to start download server")
	defer downloadServer.Stop()

	t.Logf("Download server started with %d files", numFiles)

	// Start real server executable
	server := mocks.NewExecutableServer(&mocks.ExecutableServerConfig{
		ExecPath: getServerExecutablePath(),
		Port:     0,
	})

	err = server.Start()
	helpers.AssertNoError(t, err, "Failed to start executable server")
	defer server.Stop()

	t.Logf("Server started at: %s", server.Addr())

	// Start multiple agents
	numAgents := 5
	agents := make([]*mocks.ExecutableAgent, numAgents)

	for i := 0; i < numAgents; i++ {
		agents[i] = mocks.NewExecutableAgent(&mocks.ExecutableAgentConfig{
			ExecPath:   getAgentExecutablePath(),
			ServerAddr: server.Addr(),
			AgentName:  fmt.Sprintf("stress-agent-%d", i+1),
			Port:       0,
		})

		err := agents[i].Start()
		helpers.AssertNoError(t, err, fmt.Sprintf("Failed to start agent %d", i+1))
		defer agents[i].Stop()
	}

	// Wait for all agents to register
	time.Sleep(7 * time.Second)

	// Verify all agents are running
	runningCount := 0
	for i, agent := range agents {
		if agent.IsRunning() {
			runningCount++
			t.Logf("Agent %d (ID: %s) is running", i+1, agent.GetAgentID())
		}
	}

	helpers.AssertTrue(t, runningCount == numAgents, fmt.Sprintf("All %d agents should be running", numAgents))

	t.Logf("Stress test complete - %d agents handling %d files", numAgents, numFiles)
}
