package main

import (
	"log/slog"
	"sync"
	"time"

	"internal/agents"
)

// ErrorRecord represents a single error occurrence
type ErrorRecord struct {
	Timestamp   time.Time
	TaskID      int
	SubtaskID   int
	AgentID     int
	Error       *DownloadError
	IsRetryable bool // If not retryable, it's fatal
}

// TaskErrorStats tracks error statistics for a specific task
type TaskErrorStats struct {
	TaskID          int
	TotalErrors     int
	FatalErrors     int
	RetryableErrors int
	FailedSubtasks  map[int]int // subtaskID -> error count
	AgentErrors     map[int]int // agentID -> error count
}

// AgentErrorStats tracks error statistics for a specific agent
type AgentErrorStats struct {
	AgentID         int
	TotalErrors     int
	ConsecutiveErrs int
	LastErrorTime   time.Time
	BannedUntil     time.Time
	IsBanned        bool
}

// ErrorHandlerConfig holds configuration for the error handler
type ErrorHandlerConfig struct {
	MaxErrorsPerTask     int           // Max errors before task fails
	MaxErrorsPerAgent    int           // Max errors before agent is banned
	MaxConsecutiveErrors int           // Max consecutive errors before agent is banned
	MaxSubtaskRetries    int           // Max retries per subtask before failing task
	AgentBanDuration     time.Duration // How long to ban an agent
}

// DefaultErrorHandlerConfig returns the default configuration
func DefaultErrorHandlerConfig() ErrorHandlerConfig {
	return ErrorHandlerConfig{
		MaxErrorsPerTask:     10,
		MaxErrorsPerAgent:    5,
		MaxConsecutiveErrors: 3,
		MaxSubtaskRetries:    3,
		AgentBanDuration:     5 * time.Minute,
	}
}

// ErrorHandler handles errors that occur during task and subtask execution
// There should be one ErrorHandler per server instance
type ErrorHandler struct {
	mtx          sync.RWMutex
	config       ErrorHandlerConfig
	errorRecords []ErrorRecord
	taskStats    map[int]*TaskErrorStats  // taskID -> stats
	agentStats   map[int]*AgentErrorStats // agentID -> stats

	// Reference to agent list for direct actions
	agentList agents.AgentList
}

// NewErrorHandler creates a new ErrorHandler for a server instance
func NewErrorHandler(config ErrorHandlerConfig, agentList agents.AgentList) *ErrorHandler {
	return &ErrorHandler{
		config:       config,
		errorRecords: make([]ErrorRecord, 0),
		taskStats:    make(map[int]*TaskErrorStats),
		agentStats:   make(map[int]*AgentErrorStats),
		agentList:    agentList,
	}
}

// NewErrorHandlerWithDefaults creates a new ErrorHandler with default configuration
func NewErrorHandlerWithDefaults(agentList agents.AgentList) *ErrorHandler {
	return NewErrorHandler(DefaultErrorHandlerConfig(), agentList)
}

// getOrCreateTaskStats gets or creates task stats
func (h *ErrorHandler) getOrCreateTaskStats(taskID int) *TaskErrorStats {
	if stats, ok := h.taskStats[taskID]; ok {
		return stats
	}
	stats := &TaskErrorStats{
		TaskID:         taskID,
		FailedSubtasks: make(map[int]int),
		AgentErrors:    make(map[int]int),
	}
	h.taskStats[taskID] = stats
	return stats
}

// getOrCreateAgentStats gets or creates agent stats
func (h *ErrorHandler) getOrCreateAgentStats(agentID int) *AgentErrorStats {
	if stats, ok := h.agentStats[agentID]; ok {
		return stats
	}
	stats := &AgentErrorStats{
		AgentID: agentID,
	}
	h.agentStats[agentID] = stats
	return stats
}

// HandleSubtaskError handles an error from a subtask execution.
// It logs the error, updates statistics, bans agents if needed, and fails the task if necessary.
// Returns true if the task failed (caller should stop processing), false if subtask can be retried.
func (h *ErrorHandler) HandleSubtaskError(task *taskInfo, subtask *subTaskInfo, agentID int, err *DownloadError) bool {
	if err == nil {
		return false
	}

	h.mtx.Lock()
	defer h.mtx.Unlock()

	taskID := task.id
	subtaskID := subtask.id
	retryCount := subtask.retryCount
	url := task.downloadUrl

	// Classify the error: if not retryable, it's fatal
	isRetryable := err.IsRetryable()

	// 1. Log the error
	slog.Error("Subtask error occurred",
		"taskID", taskID,
		"subtaskID", subtaskID,
		"agentID", agentID,
		"retryCount", retryCount,
		"url", url,
		"errorCode", int(err.Code),
		"isRetryable", isRetryable,
		"error", err)

	// Create error record
	record := ErrorRecord{
		Timestamp:   time.Now(),
		TaskID:      taskID,
		SubtaskID:   subtaskID,
		AgentID:     agentID,
		Error:       err,
		IsRetryable: isRetryable,
	}
	h.errorRecords = append(h.errorRecords, record)

	// Update task stats
	taskStats := h.getOrCreateTaskStats(taskID)
	taskStats.TotalErrors++
	taskStats.FailedSubtasks[subtaskID]++
	taskStats.AgentErrors[agentID]++
	if !isRetryable {
		taskStats.FatalErrors++
	} else {
		taskStats.RetryableErrors++
	}

	// Update agent stats
	agentStats := h.getOrCreateAgentStats(agentID)
	agentStats.TotalErrors++
	agentStats.ConsecutiveErrs++
	agentStats.LastErrorTime = time.Now()

	// 2. Handle fatal errors (not retryable)
	if !isRetryable {
		// Ban the agent immediately for fatal errors
		h.banAgent(agentID, "fatal error: "+err.Message)

		slog.Info("Fatal error, agent banned, subtask will retry with different agent",
			"taskID", taskID,
			"subtaskID", subtaskID,
			"agentID", agentID)
		// Don't fail task - let subtask retry with different agent
		return false
	}

	// 3. Handle retryable errors
	// Check if retry count reaches max
	if retryCount >= h.config.MaxSubtaskRetries {
		slog.Error("Subtask exceeded max retries, failing task",
			"taskID", taskID,
			"subtaskID", subtaskID,
			"retryCount", retryCount,
			"maxRetries", h.config.MaxSubtaskRetries)
		h.failTask(task, err)
		return true
	}

	// Check if agent's fail count reaches max - ban if so
	if h.shouldBanAgent(agentStats) {
		h.banAgent(agentID, "too many errors")
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
	// Update internal stats
	if stats, ok := h.agentStats[agentID]; ok {
		stats.IsBanned = true
		stats.BannedUntil = time.Now().Add(h.config.AgentBanDuration)
	}

	// Ban via agent list
	if h.agentList != nil {
		h.agentList.BanAgent(agentID, reason, time.Now().Add(h.config.AgentBanDuration))
	}

	slog.Warn("Agent banned",
		"agentID", agentID,
		"reason", reason,
		"duration", h.config.AgentBanDuration)
}

// failTask marks a task as failed
func (h *ErrorHandler) failTask(task *taskInfo, err *DownloadError) {
	slog.Error("Failing task",
		"taskID", task.id,
		"url", task.downloadUrl,
		"error", err)
	task.setError(err)
}

// TODO: this method is not used currently
// HandleTaskError handles an error at the task level.
// Returns true if the task failed, false otherwise.
func (h *ErrorHandler) HandleTaskError(task *taskInfo, err *DownloadError) bool {
	if err == nil {
		return false
	}

	h.mtx.Lock()
	defer h.mtx.Unlock()

	taskID := task.id
	url := task.downloadUrl
	isRetryable := err.IsRetryable()

	// Create error record
	record := ErrorRecord{
		Timestamp:   time.Now(),
		TaskID:      taskID,
		SubtaskID:   -1,
		AgentID:     -1,
		Error:       err,
		IsRetryable: isRetryable,
	}
	h.errorRecords = append(h.errorRecords, record)

	// Update task stats
	taskStats := h.getOrCreateTaskStats(taskID)
	taskStats.TotalErrors++
	if !isRetryable {
		taskStats.FatalErrors++
	}

	// Log the error
	slog.Error("Task error occurred",
		"taskID", taskID,
		"url", url,
		"isRetryable", isRetryable,
		"taskTotalErrors", taskStats.TotalErrors,
		"error", err)

	// Task-level errors typically fail the task if not retryable or too many errors
	if !isRetryable || taskStats.TotalErrors >= h.config.MaxErrorsPerTask {
		h.failTask(task, err)
		return true
	}

	return false
}

// ReportAgentSuccess reports a successful operation by an agent
// This resets the consecutive error counter for the agent
func (h *ErrorHandler) ReportAgentSuccess(agentID int) {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	if stats, ok := h.agentStats[agentID]; ok {
		stats.ConsecutiveErrs = 0
	}
}

// IsAgentBanned checks if an agent is currently banned
func (h *ErrorHandler) IsAgentBanned(agentID int) bool {
	h.mtx.RLock()
	defer h.mtx.RUnlock()

	if stats, ok := h.agentStats[agentID]; ok {
		if stats.IsBanned {
			// Check if ban has expired
			if time.Now().After(stats.BannedUntil) {
				return false
			}
			return true
		}
	}
	return false
}

// UnbanAgent manually unbans an agent
func (h *ErrorHandler) UnbanAgent(agentID int) {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	if stats, ok := h.agentStats[agentID]; ok {
		stats.IsBanned = false
		stats.ConsecutiveErrs = 0
		slog.Info("Agent unbanned", "agentID", agentID)
	}
}

// GetTaskStats returns the error statistics for a task
func (h *ErrorHandler) GetTaskStats(taskID int) *TaskErrorStats {
	h.mtx.RLock()
	defer h.mtx.RUnlock()

	if stats, ok := h.taskStats[taskID]; ok {
		// Return a copy to prevent concurrent modification
		statsCopy := *stats
		statsCopy.FailedSubtasks = make(map[int]int)
		statsCopy.AgentErrors = make(map[int]int)
		for k, v := range stats.FailedSubtasks {
			statsCopy.FailedSubtasks[k] = v
		}
		for k, v := range stats.AgentErrors {
			statsCopy.AgentErrors[k] = v
		}
		return &statsCopy
	}
	return nil
}

// GetAgentStats returns the error statistics for an agent
func (h *ErrorHandler) GetAgentStats(agentID int) *AgentErrorStats {
	h.mtx.RLock()
	defer h.mtx.RUnlock()

	if stats, ok := h.agentStats[agentID]; ok {
		statsCopy := *stats
		return &statsCopy
	}
	return nil
}

// CleanupTask removes error records and stats for a completed task
func (h *ErrorHandler) CleanupTask(taskID int) {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	delete(h.taskStats, taskID)

	// Remove error records for this task
	filtered := make([]ErrorRecord, 0, len(h.errorRecords))
	for _, record := range h.errorRecords {
		if record.TaskID != taskID {
			filtered = append(filtered, record)
		}
	}
	h.errorRecords = filtered
}

// shouldBanAgent determines if an agent should be banned based on its error statistics
func (h *ErrorHandler) shouldBanAgent(stats *AgentErrorStats) bool {
	// Ban if too many total errors
	if stats.TotalErrors >= h.config.MaxErrorsPerAgent {
		return true
	}

	// Ban if too many consecutive errors
	if stats.ConsecutiveErrs >= h.config.MaxConsecutiveErrors {
		return true
	}

	return false
}

// GetErrorCode returns the error code from a wrapped error
func GetErrorCode(err error) int {
	if e, ok := err.(*DownloadError); ok {
		return int(e.Code)
	}
	return 0
}

// GetErrorMessage returns a user-friendly error message
func GetErrorMessage(err error) string {
	if e, ok := err.(*DownloadError); ok {
		return e.Message
	}
	if err != nil {
		return err.Error()
	}
	return ""
}
