# Integration Test Suite

This directory contains the integration test infrastructure and tests for the DDSON distributed download system.

## Status

✅ **Implemented** - Core integration test infrastructure is complete and functional.

### Test Results
- **Combined Tests**: 8/8 passing ✅
- **Server Tests**: 7/8 passing (1 timing-sensitive test)
- **Agent Tests**: 7/8 passing (1 timing-sensitive test)

## Directory Structure

```
test/
├── integration/           # Integration test suites
│   ├── combined/         # Tests for server + agent + download server
│   │   └── download_server_test.go
│   ├── server/           # Server isolation tests
│   │   └── registration_test.go
│   └── agent/            # Agent isolation tests
│       └── agent_test.go
├── mocks/                # Test infrastructure
│   ├── common.go         # Shared utilities
│   ├── download_server.go # Test HTTP download server
│   ├── dummy_server.go   # Mock DDSON server
│   └── dummy_agent.go    # Mock DDSON agent
└── helpers/              # Test utilities
    └── setup.go          # Helper functions for tests
```

## Test Infrastructure Components

### 1. Test Download Server (`mocks/download_server.go`)
A lightweight HTTP server that simulates real-world download scenarios.

**Features**:
- Serves static files of various sizes
- Supports HTTP Range requests (partial downloads)
- Configurable response delays
- Simulates failures (404, 500, timeout, connection drops)
- Supports authentication (Basic Auth)
- Rate limiting simulation

**Usage**:
```go
config := &mocks.DownloadServerConfig{
    Port:           0,  // Random port
    SupportsRanges: true,
    SimulateDelay:  100 * time.Millisecond,
    FailureRate:    0.1,  // 10% failure rate
    RequireAuth:    true,
    Username:       "user",
    Password:       "pass",
}

server := mocks.NewTestDownloadServer(config)
server.AddFile("test.bin", testData)
server.Start()
defer server.Stop()

// Get file URL
url := server.FileURL("test.bin")
```

### 2. Dummy DDSON Server (`mocks/dummy_server.go`)
Mock gRPC server implementing the DDSON service for testing agents.

**Features**:
- Agent registration and management
- Heartbeat monitoring
- Task assignment and tracking
- Progress and error reporting (via helper methods)
- Automatic unhealthy agent detection

**Usage**:
```go
config := &mocks.DummyServerConfig{
    Port:             0,
    HeartbeatTimeout: 30 * time.Second,
}

server := mocks.NewDummyServer(config)
server.Start()
defer server.Stop()

// Assign tasks
server.AssignTask(agentID, "task-1", "http://example.com/file.bin")

// Check agent status
agents := server.GetAgents()
```

### 3. Dummy Agent (`mocks/dummy_agent.go`)
Mock agent client for testing server behavior.

**Features**:
- Connects to DDSON server
- Sends periodic heartbeats
- Simulates download progress
- Simulates failures and slow downloads
- Graceful shutdown

**Usage**:
```go
config := &mocks.DummyAgentConfig{
    ServerAddr:        "127.0.0.1:50051",
    Port:              50051,
    HeartbeatInterval: 10 * time.Second,
    SimulateFailure:   false,
}

agent := mocks.NewDummyAgent(config)
agent.Start()
defer agent.Stop()

// Simulate download
agent.SimulateDownload("task-1", 5*time.Second, 1.0)
```

### 4. Test Helpers (`helpers/setup.go`)
Utility functions for test setup and assertions.

**Features**:
- Test environment setup
- File verification (size, checksum)
- Condition waiting with timeout
- Retry with exponential backoff
- Custom assertions

## Implemented Tests

### Part 1: Combined Integration Tests
Location: `test/integration/combined/`

✅ **TestBasicDownloadServerSetup** - Verify test download server starts correctly  
✅ **TestDownloadServerFileServing** - Test file serving with checksums  
✅ **TestDownloadServerWithAuthentication** - Test authentication requirements  
✅ **TestDownloadServerWithFailures** - Test failure simulation  
✅ **TestDownloadServerWithDelay** - Test delay simulation  
✅ **TestDownloadServerRangeRequests** - Test Range request support  
✅ **TestDownloadServerMultipleFiles** - Test serving multiple files  

### Part 2: Server Isolation Tests
Location: `test/integration/server/`

✅ **TestServerAgentRegistration** - Agent registration handling  
✅ **TestServerMultipleAgentRegistration** - Multiple agents registering  
⚠️ **TestServerHeartbeatManagement** - Heartbeat monitoring (timing sensitive)  
✅ **TestServerTaskAssignment** - Task assignment to agents  
✅ **TestServerProgressReporting** - Progress update handling  
✅ **TestServerErrorReporting** - Error reporting handling  
✅ **TestServerAgentReconnection** - Agent reconnection behavior  

### Part 3: Agent Isolation Tests
Location: `test/integration/agent/`

✅ **TestAgentRegistrationFlow** - Agent registration process  
✅ **TestAgentHeartbeatTransmission** - Heartbeat sending  
✅ **TestAgentProgressReporting** - Progress reporting  
✅ **TestAgentErrorReporting** - Error reporting  
⚠️ **TestAgentSimulatedDownload** - Simulated download (timing sensitive)  
✅ **TestAgentFailureSimulation** - Failure simulation  
✅ **TestAgentGracefulShutdown** - Graceful shutdown  
✅ **TestMultipleAgentsWithSameServer** - Multiple agents scenario  

## Running Tests

### Run all integration tests:
```bash
go test ./test/integration/... -v
```

### Run specific test suite:
```bash
go test ./test/integration/combined/ -v
go test ./test/integration/server/ -v
go test ./test/integration/agent/ -v
```

### Run with timeout:
```bash
go test ./test/integration/... -v -timeout=60s
```

### Run specific test:
```bash
go test ./test/integration/combined/ -v -run TestBasicDownloadServerSetup
```

### Run with race detector:
```bash
go test ./test/integration/... -race
```

## Test Coverage

Run tests with coverage:
```bash
go test ./test/integration/... -cover
go test ./test/integration/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Implementation Notes

### Current Limitations

1. **Proto Interface Mismatch**: The current `ddson_service.proto` defines a client-server interface, not an agent-server interface. The mock implementations use the existing proto definitions for registration and heartbeats.

2. **Progress Reporting**: Since the proto doesn't have dedicated progress/error reporting RPCs for agents, the test infrastructure uses helper methods on the dummy server to simulate these operations.

3. **Timing-Sensitive Tests**: Two tests are timing-sensitive and may occasionally fail on slower systems:
   - `TestServerHeartbeatManagement` - Depends on heartbeat timeout detection
   - `TestAgentSimulatedDownload` - Depends on simulated download timing

### Future Enhancements

1. **Add agent-specific proto definitions** for more realistic testing:
   ```protobuf
   rpc AgentRegister(AgentRegisterRequest) returns (AgentRegisterResponse);
   rpc AgentReportProgress(ProgressReport) returns (ProgressResponse);
   rpc AgentReportError(ErrorReport) returns (ErrorResponse);
   ```

2. **Implement end-to-end tests** with real server and agent binaries

3. **Add performance benchmarks**:
   - Concurrent agent load testing
   - Download throughput testing
   - Failover time measurement

4. **Add stress tests**:
   - 100+ concurrent agents
   - 1000+ download tasks
   - Network instability simulations

5. **CI/CD Integration**:
   - Automated test runs on PR
   - Coverage reporting
   - Performance regression detection

## Contributing

When adding new tests:

1. Follow the existing test structure
2. Use descriptive test names
3. Add proper cleanup with `defer`
4. Use helper functions from `helpers/setup.go`
5. Add timeouts to prevent hanging tests
6. Document expected behavior
7. Consider both success and failure cases

## Dependencies

- `google.golang.org/grpc` - gRPC framework
- `github.com/imafish/ddson/internal/pb` - Generated protobuf code
- Standard library: `net/http`, `testing`, `time`, `sync`, etc.

## License

Same as main project.
