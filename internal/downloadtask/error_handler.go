package downloadtask

import (
	"log/slog"
	"sync"
	"time"

	"github.com/imafish/ddson/internal/agents"
)

// TODO: Ban agent immediately on fatal errors (non-retryable)

// TODO: this class should be extended to something like 'task execution manager' to handle task execution
// TODO: later, we should not ban the agent if it is not recoverable error, but add the agent to a banned list per task
// TODO: then we should have a global error handler to track errors across all tasks (for statistics)

// TaskErrorStats tracks error statistics for a specific task
type TaskErrorStats struct {
	Errors map[int]int // subtaskID -> error count
}

// ErrorHandlerConfig holds configuration for the error handler
type ErrorHandlerConfig struct {
	MaxSubtaskRetries int // Max retries per subtask before failing task
	AgentBanDuration  int // Duration (in seconds) to ban an agent
}

// DefaultErrorHandlerConfig returns the default configuration
func DefaultErrorHandlerConfig() ErrorHandlerConfig {
	return ErrorHandlerConfig{
		MaxSubtaskRetries: 3,
		AgentBanDuration:  300, // 5 minutes
	}
}

// ErrorHandler handles errors that occur during task and subtask execution
// There should be one ErrorHandler per server instance
type ErrorHandler struct {
	mtx    sync.RWMutex
	config ErrorHandlerConfig
	errors map[int]int // subtaskID -> error count

	// Reference to agent list for direct actions
	agentManager *agents.AgentManager
}

// NewErrorHandler creates a new ErrorHandler for a server instance
func NewErrorHandler(config ErrorHandlerConfig, agentManager *agents.AgentManager) *ErrorHandler {
	return &ErrorHandler{
		config:       config,
		errors:       make(map[int]int),
		agentManager: agentManager,
	}
}

// NewErrorHandlerWithDefaults creates a new ErrorHandler with default configuration
func NewErrorHandlerWithDefaults(agentManager *agents.AgentManager) *ErrorHandler {
	return NewErrorHandler(DefaultErrorHandlerConfig(), agentManager)
}

// HandleSubtaskError handles an error from a subtask execution.
// It logs the error, updates statistics, bans agents if needed, and fails the task if necessary.
// Returns true if the task failed (caller should stop processing), false if subtask can be retried.
func (h *ErrorHandler) HandleSubtaskError(task *Task, subtask *SubTask, agentID int, err *DownloadError) bool {
	if err == nil {
		return false
	}

	h.mtx.Lock()
	defer h.mtx.Unlock()

	taskID := task.info.ID
	subtaskID := subtask.id
	url := task.info.DownloadUrl

	// Classify the error: if not retryable, it's fatal
	h.errors[subtaskID]++
	retryCount := h.errors[subtaskID]

	// 1. Log the error
	slog.Error("Subtask error occurred",
		"taskID", taskID,
		"subtaskID", subtaskID,
		"agentID", agentID,
		"retryCount", retryCount,
		"url", url,
		"errorCode", int(err.Code),
		"error", err)

	// 2. Handle fatal errors (not retryable)
	if h.shouldBanAgent(err) {
		// Ban the agent immediately for fatal errors
		h.banAgent(agentID, "fatal error: "+err.Message)
	}

	// 3. Handle retryable errors
	// Check if retry count reaches max
	// TODO: should call h.shouldFailTask()...
	if retryCount >= h.config.MaxSubtaskRetries {
		slog.Error("Subtask exceeded max retries, failing task",
			"taskID", taskID,
			"subtaskID", subtaskID,
			"retryCount", retryCount,
			"maxRetries", h.config.MaxSubtaskRetries)
		h.failTask(task, err)
		return true
	}

	// Let subtask retry
	slog.Info("Retryable error, subtask will retry",
		"taskID", taskID,
		"subtaskID", subtaskID,
		"retryCount", retryCount)
	return false
}

// banAgent bans an agent by ID (must be called with lock held)
func (h *ErrorHandler) banAgent(agentID int, reason string) {
	// Ban via agent manager
	h.agentManager.DropAndBanAgent(agentID, time.Duration(h.config.AgentBanDuration)*time.Second, reason)

	slog.Warn("Agent banned",
		"agentID", agentID,
		"reason", reason,
		"duration", h.config.AgentBanDuration)
}

// failTask marks a task as failed
func (h *ErrorHandler) failTask(task *Task, err *DownloadError) {
	slog.Error("Failing task",
		"taskID", task.info.ID,
		"url", task.info.DownloadUrl,
		"error", err)
	task.fail(err)
}

// shouldBanAgent determines if an agent should be banned based on its error statistics
func (h *ErrorHandler) shouldBanAgent(err *DownloadError) bool {
	if err == nil {
		return false
	}

	return !err.IsRetryable()
}
