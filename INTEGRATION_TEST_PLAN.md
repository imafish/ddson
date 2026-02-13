# DDSON Integration Test Plan

## Overview
This document outlines the integration testing strategy for the DDSON distributed download system, covering both combined (server + agent) and isolated component testing.

## Test Infrastructure Requirements

### 1. Test Download Server
A lightweight HTTP server that simulates real-world download scenarios:
- **Features**:
  - Serves static files of various sizes (small: KB, medium: MB, large: GB)
  - Supports HTTP Range requests (partial downloads)
  - Configurable response delays to simulate network conditions
  - Ability to simulate failures (404, 500, timeout, connection drops)
  - Support for authentication (Basic Auth, Bearer tokens)
  - Content-Type and Content-Length headers
  - Optional rate limiting

- **Implementation Options**:
  - Go `net/http` test server with custom handlers
  - nginx with test configuration
  - Python SimpleHTTPServer with middleware

- **Test Files**:
  - `test-small.bin` (1 MB)
  - `test-medium.bin` (100 MB)
  - `test-large.bin` (1 GB)
  - `test-multipart.bin` (500 MB) - for testing partial downloads
  - `test-auth.bin` (50 MB) - requires authentication

### 2. Dummy Server (Mock DDSON Server)
For testing agents in isolation:
- **Purpose**: Simulate server gRPC API without full server logic
- **Capabilities**:
  - Accept agent registration requests
  - Send mock task assignments
  - Receive heartbeats and progress updates
  - Simulate various server responses (success, errors, timeouts)
  - Track agent behavior for assertions

- **Implementation**:
  - Go gRPC server implementing `ddson_service.proto`
  - In-memory state tracking
  - Configurable response patterns

### 3. Dummy Agent (Mock DDSON Agent)
For testing server in isolation:
- **Purpose**: Simulate agent behavior without actual download logic
- **Capabilities**:
  - Register with server
  - Send periodic heartbeats
  - Accept download tasks
  - Report simulated progress
  - Simulate various agent states (healthy, failing, slow, unresponsive)

- **Implementation**:
  - Go gRPC client using generated pb stubs
  - Configurable behavior patterns
  - Mock download completion

---

## Test Scenarios

## Part 1: Combined Integration Tests (Server + Agent + Test Download Server)

### Test 1.1: Basic End-to-End Download
**Setup**:
- Start test download server with test files
- Start DDSON server
- Start DDSON agent

**Steps**:
1. Agent registers with server
2. Submit download task via server CLI/API: `test-small.bin`
3. Server assigns task to agent
4. Agent downloads file
5. Agent reports completion

**Verification**:
- ✓ Agent successfully registers (appears in server agent list)
- ✓ Task is assigned to agent
- ✓ File is downloaded completely
- ✓ File checksum matches original
- ✓ Task status transitions: PENDING → ASSIGNED → DOWNLOADING → COMPLETED
- ✓ Database records download completion

**Success Criteria**: Complete download in < 30s, correct file integrity

---

### Test 1.2: Multi-Part Download
**Setup**:
- Test download server configured for Range requests
- DDSON server with multi-part download enabled

**Steps**:
1. Agent registers
2. Submit large file download: `test-multipart.bin`
3. Monitor download progress (should use multiple parts)

**Verification**:
- ✓ Agent requests multiple byte ranges
- ✓ Parts are downloaded concurrently
- ✓ Parts are assembled correctly
- ✓ Progress updates reflect partial completion
- ✓ Final file integrity check passes

**Success Criteria**: Download uses ≥2 concurrent parts, file integrity verified

---

### Test 1.3: Multiple Agents Load Balancing
**Setup**:
- 3 DDSON agents on different ports
- 5 download tasks queued

**Steps**:
1. All agents register
2. Submit 5 download tasks simultaneously
3. Monitor task distribution

**Verification**:
- ✓ Tasks distributed across agents
- ✓ No agent gets all tasks
- ✓ All tasks complete successfully
- ✓ Server tracks all agent heartbeats

**Success Criteria**: Load balanced within 20% deviation, all tasks complete

---

### Test 1.4: Agent Failure and Recovery
**Setup**:
- 2 agents running
- Test download server with slow file (simulated delay)

**Steps**:
1. Agents register
2. Assign download to Agent 1
3. Kill Agent 1 mid-download
4. Server detects failure via missed heartbeats
5. Server reassigns task to Agent 2

**Verification**:
- ✓ Server marks Agent 1 as unhealthy after heartbeat timeout
- ✓ Task is reassigned to Agent 2
- ✓ Agent 2 completes download
- ✓ No data corruption from interrupted download

**Success Criteria**: Failover occurs within 30s, download completes successfully

---

### Test 1.5: Download with Authentication
**Setup**:
- Test download server requiring auth
- Agent configured with credentials

**Steps**:
1. Agent registers
2. Submit download task for `test-auth.bin`
3. Agent authenticates and downloads

**Verification**:
- ✓ Agent sends proper auth headers
- ✓ Download succeeds with valid credentials
- ✓ Download fails with invalid/missing credentials

**Success Criteria**: Auth-protected files download successfully

---

### Test 1.6: Network Error Handling
**Setup**:
- Test download server configured to drop connections randomly

**Steps**:
1. Agent registers
2. Submit download task
3. Server simulates connection drops during download
4. Agent retries download

**Verification**:
- ✓ Agent detects connection failure
- ✓ Agent retries download (with exponential backoff)
- ✓ Download eventually completes
- ✓ File integrity verified

**Success Criteria**: Download completes despite 3+ simulated failures

---

### Test 1.7: Concurrent Downloads
**Setup**:
- 1 agent with concurrent download limit = 3
- Queue 5 downloads

**Steps**:
1. Agent registers
2. Submit 5 tasks
3. Monitor agent behavior

**Verification**:
- ✓ Agent downloads max 3 files concurrently
- ✓ Remaining tasks wait in queue
- ✓ As downloads complete, new ones start
- ✓ All 5 downloads complete successfully

**Success Criteria**: Concurrency limit respected, all tasks complete

---

### Test 1.8: Progress Reporting
**Setup**:
- Test download server with medium file
- Rate-limited to ~1 MB/s

**Steps**:
1. Agent registers
2. Submit download for `test-medium.bin`
3. Poll server for progress updates

**Verification**:
- ✓ Progress updates sent every N seconds (configurable)
- ✓ Progress percentage increases monotonically
- ✓ Download speed calculated correctly
- ✓ ETA estimation reasonable

**Success Criteria**: Progress updates received at least every 5s during download

---

## Part 2: Server Isolation Tests (Server + Dummy Agent + Test Download Server)

### Test 2.1: Agent Registration Handling
**Setup**:
- DDSON server running
- Dummy agent (mock)

**Steps**:
1. Dummy agent sends registration request
2. Server responds with acknowledgment
3. Verify agent added to active agent list

**Verification**:
- ✓ Server accepts registration
- ✓ Agent ID assigned/recorded
- ✓ Agent appears in server's agent list
- ✓ Agent metadata stored correctly

**Success Criteria**: Registration completes, agent tracked

---

### Test 2.2: Heartbeat Management
**Setup**:
- Server with heartbeat timeout = 30s
- 2 dummy agents

**Steps**:
1. Agents register
2. Agent 1 sends heartbeats every 10s
3. Agent 2 stops sending heartbeats after 20s
4. Observe server behavior

**Verification**:
- ✓ Server marks Agent 1 as healthy
- ✓ Server marks Agent 2 as unhealthy after timeout
- ✓ Server removes Agent 2 from available agents
- ✓ Tasks only assigned to healthy agents

**Success Criteria**: Unhealthy agent detected within timeout + 5s

---

### Test 2.3: Task Assignment Logic
**Setup**:
- 3 dummy agents with different capacities
- 10 download tasks

**Steps**:
1. Agents register with capacity info
2. Submit tasks
3. Monitor assignments

**Verification**:
- ✓ Tasks assigned based on agent capacity
- ✓ No agent overloaded beyond capacity
- ✓ Fair distribution algorithm works
- ✓ Task states tracked correctly

**Success Criteria**: Optimal task distribution, no overload

---

### Test 2.4: Agent Reconnection
**Setup**:
- Server running
- Dummy agent

**Steps**:
1. Agent registers (Agent ID: A1)
2. Agent disconnects
3. Agent reconnects with same ID
4. Server handles reconnection

**Verification**:
- ✓ Server recognizes returning agent
- ✓ Previous tasks reassigned or resumed
- ✓ No duplicate agent entries
- ✓ State synchronized

**Success Criteria**: Reconnection handled gracefully

---

### Test 2.5: Progress Update Processing
**Setup**:
- Server with active task
- Dummy agent sending progress updates

**Steps**:
1. Agent assigned task
2. Agent sends progress: 10%, 25%, 50%, 75%, 100%
3. Server processes updates

**Verification**:
- ✓ Server updates task progress in database
- ✓ Progress accessible via server API
- ✓ Progress monotonically increases
- ✓ Completion triggers task finalization

**Success Criteria**: All progress updates recorded accurately

---

### Test 2.6: Server Error Handling
**Setup**:
- Server running
- Dummy agent sending malformed requests

**Steps**:
1. Send invalid registration (missing fields)
2. Send heartbeat for non-existent agent
3. Send progress for non-existent task

**Verification**:
- ✓ Server rejects invalid requests gracefully
- ✓ Proper error messages returned
- ✓ Server doesn't crash or leak resources
- ✓ Logs show appropriate error messages

**Success Criteria**: All invalid requests handled without server failure

---

## Part 3: Agent Isolation Tests (Agent + Dummy Server + Test Download Server)

### Test 3.1: Registration Flow
**Setup**:
- Dummy server
- DDSON agent

**Steps**:
1. Agent starts and registers with dummy server
2. Dummy server accepts registration

**Verification**:
- ✓ Agent sends correct registration payload
- ✓ Agent handles server response
- ✓ Agent stores assigned ID
- ✓ Agent ready for tasks

**Success Criteria**: Registration completes successfully

---

### Test 3.2: Heartbeat Transmission
**Setup**:
- Dummy server configured to expect heartbeats every 15s
- DDSON agent

**Steps**:
1. Agent registers
2. Monitor heartbeat messages for 60s

**Verification**:
- ✓ Heartbeats sent at correct interval
- ✓ Heartbeat payload includes agent status
- ✓ Agent handles heartbeat acknowledgments
- ✓ Consistent timing (±2s variance)

**Success Criteria**: 4 heartbeats received in 60s window

---

### Test 3.3: Task Reception and Execution
**Setup**:
- Dummy server with task assignment
- Test download server
- DDSON agent

**Steps**:
1. Agent registers
2. Dummy server assigns download task
3. Agent downloads from test server
4. Agent reports completion to dummy server

**Verification**:
- ✓ Agent accepts task assignment
- ✓ Agent initiates download
- ✓ Agent downloads file correctly
- ✓ Agent reports success with file metadata

**Success Criteria**: File downloaded and reported within timeout

---

### Test 3.4: Download Part Management
**Setup**:
- Dummy server assigns multi-part task
- Test download server supporting ranges
- DDSON agent

**Steps**:
1. Agent receives task with part assignments
2. Agent downloads each part
3. Agent assembles parts

**Verification**:
- ✓ Agent requests correct byte ranges
- ✓ All parts downloaded
- ✓ Parts assembled in correct order
- ✓ Final file integrity check passes

**Success Criteria**: Multi-part download completes correctly

---

### Test 3.5: Error Reporting
**Setup**:
- Dummy server
- Test download server returning 404
- DDSON agent

**Steps**:
1. Agent assigned task with invalid URL
2. Agent attempts download
3. Agent fails and reports error

**Verification**:
- ✓ Agent detects download failure
- ✓ Agent sends error report to server
- ✓ Error includes appropriate error code/message
- ✓ Agent remains healthy and ready for next task

**Success Criteria**: Error reported within 10s of detection

---

### Test 3.6: Retry Logic
**Setup**:
- Test download server with intermittent failures
- DDSON agent with retry enabled

**Steps**:
1. Agent attempts download
2. Server returns 500 error on first 2 attempts
3. Server succeeds on 3rd attempt

**Verification**:
- ✓ Agent retries after failures
- ✓ Exponential backoff applied
- ✓ Download succeeds on retry
- ✓ Max retry limit respected (if applicable)

**Success Criteria**: Download succeeds on 3rd attempt

---

### Test 3.7: Concurrent Task Handling
**Setup**:
- Dummy server assigns 3 tasks simultaneously
- Test download server
- DDSON agent

**Steps**:
1. Agent receives 3 tasks
2. Agent downloads concurrently (if configured)
3. All complete

**Verification**:
- ✓ Agent manages concurrent downloads
- ✓ Progress tracked separately for each task
- ✓ All tasks complete successfully
- ✓ No resource leaks (file handles, goroutines)

**Success Criteria**: All 3 downloads complete within 2x single download time

---

### Test 3.8: Graceful Shutdown
**Setup**:
- Dummy server
- DDSON agent with active downloads

**Steps**:
1. Agent downloading file
2. Send shutdown signal (SIGTERM)
3. Agent shuts down

**Verification**:
- ✓ Agent saves download progress
- ✓ Agent notifies server of shutdown
- ✓ Agent cleans up resources
- ✓ Partial downloads can be resumed later

**Success Criteria**: Shutdown completes within 10s, state preserved

---

## Test Infrastructure Implementation

### Directory Structure
```
test/
├── integration/
│   ├── combined/           # Tests for Part 1
│   │   ├── e2e_test.go
│   │   ├── multipart_test.go
│   │   └── failover_test.go
│   ├── server/             # Tests for Part 2
│   │   ├── registration_test.go
│   │   └── heartbeat_test.go
│   ├── agent/              # Tests for Part 3
│   │   ├── download_test.go
│   │   └── retry_test.go
│   └── fixtures/
│       └── testdata/       # Test files
├── mocks/
│   ├── dummy_server.go     # Mock DDSON server
│   ├── dummy_agent.go      # Mock DDSON agent
│   └── download_server.go  # Test HTTP download server
└── helpers/
    ├── setup.go            # Test setup utilities
    └── assertions.go       # Custom test assertions
```

### Test Download Server Implementation
```go
// test/mocks/download_server.go
type TestDownloadServer struct {
    httpServer   *http.Server
    config       *DownloadServerConfig
    files        map[string][]byte
}

type DownloadServerConfig struct {
    Port            int
    SupportsRanges  bool
    SimulateDelay   time.Duration
    FailureRate     float64
    RequireAuth     bool
}

func NewTestDownloadServer(config *DownloadServerConfig) *TestDownloadServer
func (s *TestDownloadServer) AddFile(name string, data []byte)
func (s *TestDownloadServer) Start() error
func (s *TestDownloadServer) Stop() error
```

### Dummy Server Implementation
```go
// test/mocks/dummy_server.go
type DummyServer struct {
    grpcServer      *grpc.Server
    registeredAgents map[string]*AgentInfo
    assignedTasks   map[string]*Task
}

func NewDummyServer() *DummyServer
func (s *DummyServer) Start(port int) error
func (s *DummyServer) Stop()
func (s *DummyServer) AssignTask(agentID string, task *Task) error
func (s *DummyServer) GetAgents() []*AgentInfo
```

### Dummy Agent Implementation
```go
// test/mocks/dummy_agent.go
type DummyAgent struct {
    id              string
    serverAddr      string
    heartbeatTicker *time.Ticker
}

func NewDummyAgent(serverAddr string) *DummyAgent
func (a *DummyAgent) Register() error
func (a *DummyAgent) StartHeartbeat(interval time.Duration)
func (a *DummyAgent) ReportProgress(taskID string, progress int) error
func (a *DummyAgent) SimulateFailure()
```

---

## Test Execution Plan

### Phase 1: Infrastructure Setup (Week 1)
- [ ] Implement test download server
- [ ] Implement dummy server
- [ ] Implement dummy agent
- [ ] Create test fixtures and helper utilities
- [ ] Set up CI/CD integration test pipeline

### Phase 2: Combined Integration Tests (Week 2)
- [ ] Implement and run Tests 1.1-1.4 (basic functionality)
- [ ] Implement and run Tests 1.5-1.8 (advanced scenarios)
- [ ] Document failures and fixes

### Phase 3: Isolation Tests (Week 3)
- [ ] Implement and run Part 2 tests (server isolation)
- [ ] Implement and run Part 3 tests (agent isolation)
- [ ] Validate component boundaries

### Phase 4: Performance and Stress Testing (Week 4)
- [ ] 100 concurrent agents
- [ ] 1000+ download tasks
- [ ] Large file downloads (10+ GB)
- [ ] Network instability simulations
- [ ] Resource usage monitoring

### Phase 5: Documentation and Automation (Week 5)
- [ ] Document test results
- [ ] Create automated test suite
- [ ] Integration with CI/CD
- [ ] Test coverage report
- [ ] Production readiness checklist

---

## Success Metrics

### Coverage Goals
- **Unit Test Coverage**: 80%+ for all packages
- **Integration Test Coverage**: All critical paths tested
- **Edge Cases**: 50+ edge cases covered

### Performance Benchmarks
- Agent registration: < 100ms
- Task assignment: < 50ms per agent
- Download throughput: 80%+ of network bandwidth
- Heartbeat overhead: < 1% CPU usage
- Memory usage: < 100MB per agent (idle)

### Reliability Targets
- 99.9% uptime for server
- Zero data corruption
- Failover time: < 30s
- Error recovery: 95%+ success rate

---

## Risk Mitigation

### Known Risks
1. **Network flakiness in CI**: Use retry logic in tests
2. **Timing-sensitive tests**: Add tolerance windows
3. **Resource cleanup**: Use defer and t.Cleanup()
4. **Race conditions**: Run tests with -race flag
5. **Port conflicts**: Use dynamic port allocation

### Monitoring During Tests
- CPU and memory usage
- Network bandwidth utilization
- Goroutine counts
- Open file descriptors
- gRPC connection states
- Database connection pool status

---

## Tools and Technologies

### Testing Frameworks
- **Go testing**: Built-in test framework
- **testify**: Assertions and mocking
- **gomock**: Generate mocks from interfaces
- **httptest**: HTTP test servers

### Monitoring
- **pprof**: Profiling and resource monitoring
- **prometheus**: Metrics collection
- **grafana**: Visualization (optional)

### CI/CD
- **GitHub Actions** / **GitLab CI** / **Jenkins**
- Automated test runs on PR and merge
- Nightly extended integration tests
- Performance regression detection

---

## Next Steps

1. **Review this plan** with the team
2. **Prioritize test scenarios** based on risk
3. **Set up development environment** for integration testing
4. **Implement test infrastructure** (mocks and fixtures)
5. **Begin Phase 1** of execution plan
6. **Iterate and refine** based on findings

---

## Appendix

### Test Data Files
Generate test files with:
```bash
# Small file (1 MB)
dd if=/dev/urandom of=test-small.bin bs=1M count=1

# Medium file (100 MB)
dd if=/dev/urandom of=test-medium.bin bs=1M count=100

# Large file (1 GB)
dd if=/dev/urandom of=test-large.bin bs=1M count=1024

# Calculate checksums
sha256sum test-*.bin > checksums.txt
```

### Environment Variables for Tests
```bash
export DDSON_TEST_SERVER_PORT=8080
export DDSON_TEST_DOWNLOAD_PORT=9090
export DDSON_TEST_GRPC_PORT=50051
export DDSON_TEST_TIMEOUT=300s
export DDSON_TEST_DATA_DIR=./test/fixtures/testdata
```

### Quick Test Run Commands
```bash
# Run all integration tests
go test ./test/integration/... -v

# Run specific test suite
go test ./test/integration/combined -v -run TestE2E

# Run with race detector
go test ./test/integration/... -race

# Run with coverage
go test ./test/integration/... -cover -coverprofile=coverage.out
```
