package mocks

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Pre-compiled regex patterns for extracting server address from logs
var serverAddrPatterns = []*regexp.Regexp{
	regexp.MustCompile(`address=\[::]:(\d+)`),                     // address=[::]:5510
	regexp.MustCompile(`address=([0-9.]+):(\d+)`),                 // address=0.0.0.0:5510
	regexp.MustCompile(`[Ll]istening on\s+:?(\d+)`),               // listening on :5510
	regexp.MustCompile(`[Ll]istening on\s+([0-9.]+):(\d+)`),       // listening on 0.0.0.0:5510
	regexp.MustCompile(`[Ss]erver.*(?:on|at)\s+([0-9.]+):(\d+)`),  // Server listening on 0.0.0.0:5510
	regexp.MustCompile(`[Ss]tarted.*(?:on|at)\s+([0-9.]+):(\d+)`), // Started on 0.0.0.0:5510
	regexp.MustCompile(`port[=:\s]+(\d+)`),                        // port=5510
}

// ExecutableServer runs the actual ddson_server executable
type ExecutableServer struct {
	execPath     string
	port         int
	workspaceDir string
	cmd          *exec.Cmd
	addr         string
	stdout       io.ReadCloser
	stderr       io.ReadCloser
	mu           sync.Mutex
	running      bool
	ready        atomic.Bool // Use atomic for lock-free reads
}

// ExecutableServerConfig configures an executable server
type ExecutableServerConfig struct {
	ExecPath     string // Path to ddson_server executable
	Port         int    // Server port (0 for auto)
	WorkspaceDir string // Workspace directory (empty for temp)
}

// NewExecutableServer creates a new executable server wrapper
func NewExecutableServer(config *ExecutableServerConfig) *ExecutableServer {
	if config.Port == 0 {
		config.Port = 0 // Will be auto-assigned
	}

	if config.WorkspaceDir == "" {
		// Create temp workspace directory
		config.WorkspaceDir = filepath.Join(os.TempDir(), fmt.Sprintf("ddson-test-server-%d", time.Now().UnixNano()))
	}

	return &ExecutableServer{
		execPath:     config.ExecPath,
		port:         config.Port,
		workspaceDir: config.WorkspaceDir,
	}
}

// Start starts the server executable
func (es *ExecutableServer) Start() error {
	es.mu.Lock()

	if es.running {
		es.mu.Unlock()
		return fmt.Errorf("server already running")
	}

	// Create workspace directory
	if err := os.MkdirAll(es.workspaceDir, 0755); err != nil {
		es.mu.Unlock()
		return fmt.Errorf("failed to create workspace directory: %v", err)
	}

	// Build command
	args := []string{}

	// Always pass the --port flag. When port is 0, the server will allocate a random port.
	// When port > 0, use the specified port.
	args = append(args, "--port", strconv.Itoa(es.port))

	es.cmd = exec.Command(es.execPath, args...)

	// Set environment to override HOME directory for workspace isolation
	// The server uses ~/workspace_ddson, so we override HOME
	env := append(os.Environ(), fmt.Sprintf("HOME=%s", es.workspaceDir))
	es.cmd.Env = env

	// Capture stdout and stderr
	var err error
	es.stdout, err = es.cmd.StdoutPipe()
	if err != nil {
		es.mu.Unlock()
		return fmt.Errorf("failed to get stdout pipe: %v", err)
	}

	es.stderr, err = es.cmd.StderrPipe()
	if err != nil {
		es.mu.Unlock()
		return fmt.Errorf("failed to get stderr pipe: %v", err)
	}

	// Start the process
	if err := es.cmd.Start(); err != nil {
		es.mu.Unlock()
		return fmt.Errorf("failed to start server: %v", err)
	}

	es.running = true

	// Start log readers
	go es.readLogs(es.stdout, "STDOUT")
	go es.readLogs(es.stderr, "STDERR")

	// Release the mutex before waiting for ready (long operation)
	es.mu.Unlock()

	// Wait for server to be ready (parse address from logs)
	if err := es.waitForReady(); err != nil {
		es.Stop()
		return fmt.Errorf("failed to start: %v", err)
	}

	log.Printf("ExecutableServer: Started (Addr: %s, PID: %d, Workspace: %s)", es.addr, es.cmd.Process.Pid, es.workspaceDir)

	return nil
}

// Stop stops the server
func (es *ExecutableServer) Stop() error {
	es.mu.Lock()
	defer es.mu.Unlock()

	if !es.running {
		return nil
	}

	log.Printf("ExecutableServer: Stopping (Addr: %s)", es.addr)

	if es.cmd != nil && es.cmd.Process != nil {
		// Send interrupt signal
		if err := es.cmd.Process.Signal(os.Interrupt); err != nil {
			log.Printf("ExecutableServer: Failed to interrupt process: %v", err)
			// Force kill if interrupt fails
			es.cmd.Process.Kill()
		}

		// Wait for process to exit (with timeout)
		done := make(chan error, 1)
		go func() {
			done <- es.cmd.Wait()
		}()

		select {
		case <-done:
			// Process exited
		case <-time.After(5 * time.Second):
			log.Printf("ExecutableServer: Process did not exit, force killing")
			es.cmd.Process.Kill()
			<-done
		}
	}

	// Cleanup workspace directory
	if es.workspaceDir != "" {
		if err := os.RemoveAll(es.workspaceDir); err != nil {
			log.Printf("ExecutableServer: Failed to remove workspace: %v", err)
		}
	}

	es.running = false
	es.ready.Store(false)
	return nil
}

// IsRunning returns whether the server is running
func (es *ExecutableServer) IsRunning() bool {
	es.mu.Lock()
	defer es.mu.Unlock()
	return es.running
}

// IsReady returns whether the server is ready to accept connections
func (es *ExecutableServer) IsReady() bool {
	return es.ready.Load()
}

// Addr returns the server address
func (es *ExecutableServer) Addr() string {
	es.mu.Lock()
	defer es.mu.Unlock()
	return es.addr
}

// GetWorkspaceDir returns the workspace directory
func (es *ExecutableServer) GetWorkspaceDir() string {
	return es.workspaceDir
}

// readLogs reads and logs output from the server
func (es *ExecutableServer) readLogs(reader io.ReadCloser, prefix string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("ExecutableServer[%s]: %s", prefix, line)

		// Try to extract server address from log line
		es.extractServerAddr(line)
	}
}

// extractServerAddr extracts server address from log lines
func (es *ExecutableServer) extractServerAddr(line string) {
	// Quick check without mutex to avoid contention
	if es.ready.Load() {
		return
	}

	// Use pre-compiled regex patterns
	for _, re := range serverAddrPatterns {
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			es.mu.Lock()
			// Double-check after acquiring lock
			if es.addr == "" {
				// Extract port
				var port string
				if len(matches) == 2 {
					// Only port captured
					port = matches[1]
					es.addr = fmt.Sprintf("localhost:%s", port)
				} else if len(matches) == 3 {
					// Host and port captured
					host := matches[1]
					port = matches[2]
					if host == "0.0.0.0" || host == "" {
						host = "localhost"
					}
					es.addr = fmt.Sprintf("%s:%s", host, port)
				}
				es.ready.Store(true)
				log.Printf("ExecutableServer: Extracted server address: %s", es.addr)
			}
			es.mu.Unlock()
			return
		}
	}
}

// waitForReady waits for the server to be ready
func (es *ExecutableServer) waitForReady() error {
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for server to be ready")
		case <-ticker.C:
			if es.ready.Load() {
				return nil
			}

			// Also check if process has exited
			if es.cmd.ProcessState != nil && es.cmd.ProcessState.Exited() {
				return fmt.Errorf("process exited before becoming ready")
			}
		}
	}
}

// WaitForReady blocks until the server is ready or timeout
func (es *ExecutableServer) WaitForReady(timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timeout waiting for server to be ready")
		case <-ticker.C:
			if es.IsReady() {
				return nil
			}
		}
	}
}
