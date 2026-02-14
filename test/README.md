# Test Suite

This directory contains the test infrastructure and tests for the DDSON distributed download system.

## Status

✅ **Organized** - Tests are separated into integration tests (one real component) and e2e tests (all real components).

### Test Architecture

Tests are organized by isolation level:

- **Integration Tests** (`test/integration/`): Test individual components with real executable + fake dependencies
  - **Server Tests**: Real server executable + fake agent + fake download server
  - **Agent Tests**: Real agent executable + fake server + fake download server

- **E2E Tests** (`test/e2e/`): Test complete system with all real executables
  - Real server executable + real agent executable + fake download server

## Directory Structure

```
test/
├── integration/           # Integration tests (one real component + fakes)
│   ├── server/           # Server integration tests
│   │   ├── server_test.go            # Real server + fake components
│   │   └── registration_test.go      # Legacy mock-based tests
│   └── agent/            # Agent integration tests
│       ├── agent_test.go             # Real agent + fake components
│       └── agent_test.go.bak         # Backup
│
├── e2e/                   # End-to-end tests (all real components)
│   └── e2e_test.go                    # Real server + real agent + fake download
│
├── combined/              # Legacy tests (deprecated)
│   ├── download_server_test.go       # Mock-based tests
│   ├── real_integration_test.go      # In-process tests
│   └── e2e_with_executables_test.go  # Old E2E tests (moved to test/e2e/)
├── mocks/                # Test infrastructure
│   ├── common.go              # Shared utilities
│   ├── download_server.go     # Test HTTP download server (fake)
│   ├── dummy_server.go        # Mock DDSON server (fake)
│   ├── dummy_agent.go         # Mock DDSON agent (fake)
│   ├── executable_server.go   # Real server executable wrapper
│   ├── executable_agent.go    # Real agent executable wrapper
├── bin/                   # Test executables
│   ├── ddson_server       # Compiled server for testing
│   └── ddson_client       # Compiled agent for testing
└── helpers/              # Test utilities
    └── setup.go          # Helper functions for tests
```

## Test Infrastructure Components

### 1. Test Download Server (`mocks/download_server.go`) - **Fake**
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

### 2. Executable Server (`mocks/executable_server.go`) - **Real** ⭐
Wrapper for running the actual `ddson_server` executable in tests.

**Features**:
- Runs real compiled server binary
- Automatic port allocation
- Isolated workspace per instance
- Log capture and parsing
- Clean process management and cleanup

**Usage**:
```go
server := mocks.NewExecutableServer(&mocks.ExecutableServerConfig{
    ExecPath: "/path/to/ddson_server",
    Port:     0,  // Auto-assign
})

server.Start()
defer server.Stop()

// Get server address
addr := server.Addr()  // e.g., "localhost:54321"
```

### 3. Executable Agent (`mocks/executable_agent.go`) - **Real** ⭐
Wrapper for running the actual `ddson_client` executable in tests.

**Features**:
- Runs real compiled agent binary
- Automatic agent ID extraction from logs
- Supports agent-mode execution
- Log capture and parsing
- Clean process management

**Usage**:
```go
agent := mocks.NewExecutableAgent(&mocks.ExecutableAgentConfig{
    ExecPath:   "/path/to/ddson_client",
    ServerAddr: serverAddr,
    AgentName:  "test-agent-1",
    Port:       0,  // Auto-assign
})

agent.Start()
defer agent.Stop()

// Get agent ID
agentID := agent.GetAgentID()
```

### 4. Dummy DDSON Server (`mocks/dummy_server.go`) - **Fake**
Mock gRPC server implementing the DDSON service for testing agents in isolation.

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

### 5. Dummy Agent (`mocks/dummy_agent.go`) - **Fake**
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

### 4. Real Server Wrapper (`mocks/real_server.go`)
Wraps the actual DDSON server implementation for testing.

**Features**:
- Uses real agent manager and task manager
- Temporary workspace directory for isolation
- Access to internal components for verification
- Proper cleanup after tests

**Usage**:
```go
server := mocks.NewRealServer(&mocks.RealServerConfig{
    Port:     0,  // Random port
    LogLevel: slog.LevelError,  // Quiet in tests
})
server.Start()
defer server.Stop()

// Access internal components
agentManager := server.GetAgentManager()
taskManager := server.GetTaskManager()
```

### 5. Real Agent Wrapper (`mocks/real_agent.go`)
Wraps the actual DDSON agent implementation for testing.

**Features**:
- Uses real agent code from cmd/client
- Automatic registration and heartbeats
- Connection management
- Proper cleanup

**Usage**:
```go
agent := mocks.NewRealAgent(&mocks.RealAgentConfig{
    ServerAddr:        "127.0.0.1:5510",
    AgentName:         "test-agent",
    HeartbeatInterval: 2 * time.Second,
})
agent.Start()
defer agent.Stop()
```

### 6. Test Helpers (`helpers/setup.go`)
Utility functions for test setup and assertions.

**Features**:
- Test environment setup
- File verification (size, checksum)
- Condition waiting with timeout
- Retry with exponential backoff
- Custom assertions

## Implemented Tests

### Test Categories

#### 1. Integration Tests - Server (`test/integration/server/`)
**Real Server + Fake Components**

Uses: `ExecutableServer` + `DummyAgent` + `DownloadServer`

✅ **TestExecutableServerAgentRegistration** - Agent registration with real server  
✅ **TestExecutableServerMultipleAgents** - Multiple agents registering  
✅ **TestExecutableServerHeartbeatHandling** - Heartbeat handling  
✅ **TestExecutableServerAgentReconnection** - Agent reconnection  
✅ **TestExecutableServerWithDownloadServer** - Integration with download server  
✅ **TestExecutableServerStopsCleanly** - Clean shutdown  
✅ **TestExecutableServerWorkspaceIsolation** - Workspace isolation  

**7 tests** - Location: `test/integration/server/server_test.go`

#### 2. Integration Tests - Agent (`test/integration/agent/`)
**Real Agent + Fake Components**

Uses: `ExecutableAgent` + `DummyServer` + `DownloadServer`

✅ **TestExecutableAgentRegistration** - Agent registration  
✅ **TestExecutableAgentHeartbeat** - Heartbeat sending  
✅ **TestExecutableAgentMultipleAgents** - Multiple agent instances  
✅ **TestExecutableAgentWithDownloadServer** - Download task handling  
✅ **TestExecutableAgentStopsCleanly** - Clean shutdown  
✅ **TestExecutableAgentReconnection** - Reconnection handling  
✅ **TestExecutableAgentWithAuth** - Authenticated downloads  

**8 tests** - Location: `test/integration/agent/agent_test.go`

#### 3. End-to-End Tests (`test/e2e/`)
**Real Server + Real Agent + Fake Download Server**

Uses: `ExecutableServer` + `ExecutableAgent` + `DownloadServer`

✅ **TestE2EBasicDownload** - Basic end-to-end download  
✅ **TestE2EMultipleAgents** - Multiple agents coordination  
✅ **TestE2ELargeFileDownload** - Large file handling (10 MB)  
✅ **TestE2EWithAuthentication** - Authenticated download flow  
✅ **TestE2EPartialDownload** - Partial/range downloads  
✅ **TestE2EAgentFailover** - Agent failover scenario  
✅ **TestE2EStressTest** - System under load (5 agents, 5 files)  

**8 tests** - Location: `test/e2e/e2e_test.go`

## Test Architecture

### Component Matrix

| Test Type        | Server               | Agent               | Download Server     | Purpose                          |
| ---------------- | -------------------- | ------------------- | ------------------- | -------------------------------- |
| **Server Tests** | ✅ Real Executable    | ❌ Fake (DummyAgent) | ❌ Fake (TestServer) | Test server logic in isolation   |
| **Agent Tests**  | ❌ Fake (DummyServer) | ✅ Real Executable   | ❌ Fake (TestServer) | Test agent logic in isolation    |
| **E2E Tests**    | ✅ Real Executable    | ✅ Real Executable   | ❌ Fake (TestServer) | Test complete system integration |

### Benefits of This Approach

1. **True Integration Testing**: Uses actual compiled executables, not in-process mocks
2. **Process Isolation**: Each test runs independent server/agent processes
3. **Realistic Environment**: Tests subprocess management, IPC, signal handling
4. **Parallel Execution**: Tests can run in parallel with `t.Parallel()`
5. **No Docker Required**: Simple subprocess execution with temp directories
6. **Easy Debugging**: Can attach debuggers to running test processes
7. **Fast Iteration**: No Docker overhead, quick test execution

## Running Tests

### Build test executables first:
```bash
cd test
./build_test_binaries.sh
```

This creates:
- `test/bin/ddson_server` - Server executable for testing
- `test/bin/ddson_client` - Agent executable for testing

### Run all tests:
```bash
# All tests (integration + e2e)
go test ./test/integration/... ./test/e2e/... -v

# Or separately
go test ./test/integration/... -v
go test ./test/e2e/... -v
```

### Run specific test suite:
```bash
# Server integration tests (real server + fake components)
go test ./test/integration/server/ -v

# Agent integration tests (real agent + fake components)
go test ./test/integration/agent/ -v

# End-to-end tests (all real components)
go test ./test/e2e/ -v
```

### Run with parallel execution:
```bash
go test ./test/integration/... -v -parallel=4
```

### Run with timeout:
```bash
go test ./test/integration/... -v -timeout=120s
```

### Run specific test:
```bash
go test ./test/integration/combined/ -v -run TestE2EBasicDownload
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
