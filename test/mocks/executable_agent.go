package mocks

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// ExecutableAgent runs the actual ddson_client executable as an agent
type ExecutableAgent struct {
	execPath   string
	serverAddr string
	agentName  string
	port       int
	cmd        *exec.Cmd
	agentID    string
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	mu         sync.Mutex
	running    bool
	registered atomic.Bool
}

// ExecutableAgentConfig configures an executable agent
type ExecutableAgentConfig struct {
	ExecPath   string // Path to ddson_client executable
	ServerAddr string
	AgentName  string
	Port       int // Agent service port (0 for auto)
}

// NewExecutableAgent creates a new executable agent wrapper
func NewExecutableAgent(config *ExecutableAgentConfig) *ExecutableAgent {
	if config.Port == 0 {
		config.Port = 0 // Will be auto-assigned
	}

	return &ExecutableAgent{
		execPath:   config.ExecPath,
		serverAddr: config.ServerAddr,
		agentName:  config.AgentName,
		port:       config.Port,
	}
}

// Start starts the agent executable
func (ea *ExecutableAgent) Start() error {
	ea.mu.Lock()

	if ea.running {
		ea.mu.Unlock()
		return fmt.Errorf("agent already running")
	}

	// Build command
	args := []string{
		"--addr", ea.serverAddr,
		"--name", ea.agentName,
		"--port", strconv.Itoa(ea.port), // Always pass port (0 = automatic allocation)
	}

	ea.cmd = exec.Command(ea.execPath, args...)

	// Capture stdout and stderr
	var err error
	ea.stdout, err = ea.cmd.StdoutPipe()
	if err != nil {
		ea.mu.Unlock()
		return fmt.Errorf("failed to get stdout pipe: %v", err)
	}

	ea.stderr, err = ea.cmd.StderrPipe()
	if err != nil {
		ea.mu.Unlock()
		return fmt.Errorf("failed to get stderr pipe: %v", err)
	}

	// Start the process
	if err := ea.cmd.Start(); err != nil {
		ea.mu.Unlock()
		return fmt.Errorf("failed to start agent: %v", err)
	}

	ea.running = true

	// Start log readers
	go ea.readLogs(ea.stdout, "STDOUT")
	go ea.readLogs(ea.stderr, "STDERR")

	// Release mutex before waiting (long operation)
	ea.mu.Unlock()

	// Wait for registration (parse agent ID from logs)
	if err := ea.waitForRegistration(); err != nil {
		ea.Stop()
		return fmt.Errorf("failed to register: %v", err)
	}

	log.Printf("ExecutableAgent: Started %s (ID: %s, PID: %d)", ea.agentName, ea.agentID, ea.cmd.Process.Pid)

	return nil
}

// Stop stops the agent
func (ea *ExecutableAgent) Stop() error {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	if !ea.running {
		return nil
	}

	log.Printf("ExecutableAgent: Stopping %s (ID: %s)", ea.agentName, ea.agentID)

	if ea.cmd != nil && ea.cmd.Process != nil {
		// Send interrupt signal
		if err := ea.cmd.Process.Signal(os.Interrupt); err != nil {
			log.Printf("ExecutableAgent: Failed to interrupt process: %v", err)
			// Force kill if interrupt fails
			ea.cmd.Process.Kill()
		}

		// Wait for process to exit (with timeout)
		done := make(chan error, 1)
		go func() {
			done <- ea.cmd.Wait()
		}()

		select {
		case <-done:
			// Process exited
		case <-time.After(5 * time.Second):
			log.Printf("ExecutableAgent: Process did not exit, force killing")
			ea.cmd.Process.Kill()
			<-done
		}
	}

	ea.running = false
	return nil
}

// IsRunning returns whether the agent is running
func (ea *ExecutableAgent) IsRunning() bool {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	return ea.running
}

// GetAgentID returns the agent ID
func (ea *ExecutableAgent) GetAgentID() string {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	return ea.agentID
}

// GetAgentName returns the agent name
func (ea *ExecutableAgent) GetAgentName() string {
	return ea.agentName
}

// readLogs reads and logs output from the agent
func (ea *ExecutableAgent) readLogs(reader io.ReadCloser, prefix string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("ExecutableAgent[%s][%s]: %s", ea.agentName, prefix, line)

		// Try to extract agent ID from log line
		if ea.agentID == "" {
			ea.extractAgentID(line)
		}
	}
}

// extractAgentID extracts agent ID from log lines
func (ea *ExecutableAgent) extractAgentID(line string) {
	// Quick check to avoid unnecessary work
	if ea.registered.Load() {
		return
	}

	// Look for patterns like "ID: 1" or "id=1" in logs
	patterns := []string{
		`ID:\s*(\d+)`,
		`id=(\d+)`,
		`agent_id=(\d+)`,
		`registered.*id[=:\s]+(\d+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			ea.mu.Lock()
			if ea.agentID == "" {
				ea.agentID = matches[1]
				ea.registered.Store(true)
				log.Printf("ExecutableAgent: Extracted agent ID: %s", ea.agentID)
			}
			ea.mu.Unlock()
			return
		}
	}
}

// waitForRegistration waits for the agent to register
func (ea *ExecutableAgent) waitForRegistration() error {
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for registration")
		case <-ticker.C:
			ea.mu.Lock()
			id := ea.agentID
			ea.mu.Unlock()

			if id != "" {
				return nil
			}

			// Also check if process has exited
			if ea.cmd.ProcessState != nil && ea.cmd.ProcessState.Exited() {
				return fmt.Errorf("process exited before registration")
			}
		}
	}
}
