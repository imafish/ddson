package downloadtask

import (
	"log/slog"
	"sync"
	"time"
)

// downloadProgressCollector collects and reports download status.
// It collects download speed, subtasks, progress, errors, etc.
type downloadProgressCollector struct {
	task *Task
	mtx  sync.Mutex

	// Per-subtask statistics
	subtaskStats map[int]*subtaskStat

	// Per-agent statistics
	agentStats map[int]*agentStat

	// Overall statistics
	totalDownloadedBytes int64
	taskStartTime        time.Time
	runningSubtasks      int
	completedSubtasks    int

	// For speed calculation (sliding window of 1 minute)
	speedSamples    []speedSample
	speedWindowSize time.Duration
	lastSpeedUpdate time.Time
	currentSpeedBPS int // bytes per second
}

type subtaskStat struct {
	subtaskID       int
	agentID         int // current agent working on this subtask
	downloadedBytes int64
	startTime       time.Time
	isCompleted     bool
}

type agentStat struct {
	agentID              int
	totalDownloadedBytes int64
	downloadCount        int // number of downloads completed by this agent
	totalDownloadTime    time.Duration
	currentSubtaskID     int // -1 if idle
}

type speedSample struct {
	timestamp time.Time
	bytes     int64
}

func newDownloadStatusCollector(task *Task) *downloadProgressCollector {
	return &downloadProgressCollector{
		task:            task,
		subtaskStats:    make(map[int]*subtaskStat),
		agentStats:      make(map[int]*agentStat),
		taskStartTime:   time.Now(),
		speedSamples:    make([]speedSample, 0),
		speedWindowSize: time.Minute,
		lastSpeedUpdate: time.Now(),
	}
}

func (dsc *downloadProgressCollector) onSubtaskProgress(subtaskID int, downloadedBytes int64) {
	dsc.mtx.Lock()
	defer dsc.mtx.Unlock()

	slog.Debug("Subtask progress", "subtaskID", subtaskID, "bytes", downloadedBytes)

	now := time.Now()

	// Update subtask stats
	stat, exists := dsc.subtaskStats[subtaskID]
	if !exists {
		stat = &subtaskStat{
			subtaskID: subtaskID,
			startTime: now,
		}
		dsc.subtaskStats[subtaskID] = stat
	}
	stat.downloadedBytes += downloadedBytes

	// Update agent stats if we know which agent is handling this
	if stat.agentID > 0 {
		if agentStat, ok := dsc.agentStats[stat.agentID]; ok {
			agentStat.totalDownloadedBytes += downloadedBytes
		}
	}

	// Update total downloaded bytes
	dsc.totalDownloadedBytes += downloadedBytes

	// Add speed sample
	dsc.speedSamples = append(dsc.speedSamples, speedSample{
		timestamp: now,
		bytes:     downloadedBytes,
	})

	// Calculate speed if enough time has passed (at least 500ms between updates)
	if now.Sub(dsc.lastSpeedUpdate) >= 500*time.Millisecond {
		dsc.calculateSpeedLocked(now)
		dsc.lastSpeedUpdate = now
		dsc.updateTaskStatusLocked()
	}
}

func (dsc *downloadProgressCollector) onSubtaskStart(subtaskID int, agentID int) {
	dsc.mtx.Lock()
	defer dsc.mtx.Unlock()

	now := time.Now()

	// Get or create subtask stats
	stat, exists := dsc.subtaskStats[subtaskID]
	if !exists {
		stat = &subtaskStat{
			subtaskID: subtaskID,
		}
		dsc.subtaskStats[subtaskID] = stat
	}

	// Set up for this run (OnSubtaskError already wiped data if this is a retry)
	stat.agentID = agentID
	stat.startTime = now
	stat.isCompleted = false

	// Update running count
	dsc.runningSubtasks++

	// Update agent stats
	agentStat := dsc.getOrCreateAgentStatLocked(agentID)
	agentStat.currentSubtaskID = subtaskID

	// Update task status
	runningCount := dsc.runningSubtasks
	dsc.mtx.Unlock()
	dsc.task.updateAndSendTaskStatus("DownloadingParts", runningCount)
	dsc.mtx.Lock()
}

func (dsc *downloadProgressCollector) onSubtaskError(subtaskID int, err *DownloadError) {
	dsc.mtx.Lock()
	defer dsc.mtx.Unlock()

	stat, exists := dsc.subtaskStats[subtaskID]
	if !exists {
		slog.Error("Subtask error for unknown subtask", "subtaskID", subtaskID)
		return
	}

	slog.Warn("Subtask error, wiping progress",
		"subtaskID", subtaskID,
		"agentID", stat.agentID,
		"wipedBytes", stat.downloadedBytes,
		"error", err)

	// Wipe this subtask's download data from total
	dsc.totalDownloadedBytes -= stat.downloadedBytes

	// Wipe from agent stats
	if stat.agentID > 0 {
		if agentStat, ok := dsc.agentStats[stat.agentID]; ok {
			agentStat.totalDownloadedBytes -= stat.downloadedBytes
			agentStat.currentSubtaskID = -1
		}
	}

	// Update running count
	dsc.runningSubtasks--

	// Reset subtask stats (but keep it in map for potential retry)
	stat.downloadedBytes = 0
	stat.agentID = 0

	// Update task status
	runningCount := dsc.runningSubtasks
	totalBytes := dsc.totalDownloadedBytes
	dsc.mtx.Unlock()
	dsc.task.updateAndSendTaskStatus("DownloadingParts", runningCount, "TotalDownloadedBytes", totalBytes)
	dsc.mtx.Lock()
}

func (dsc *downloadProgressCollector) onSubtaskCompleted(subtaskID int, filePath string) {
	dsc.mtx.Lock()
	defer dsc.mtx.Unlock()

	now := time.Now()

	stat, exists := dsc.subtaskStats[subtaskID]
	if !exists {
		slog.Error("Subtask completed for unknown subtask", "subtaskID", subtaskID)
		return
	}

	duration := now.Sub(stat.startTime)
	slog.Info("Subtask completed",
		"subtaskID", subtaskID,
		"agentID", stat.agentID,
		"bytes", stat.downloadedBytes,
		"duration", duration,
		"file", filePath)

	// Update counters
	stat.isCompleted = true
	dsc.runningSubtasks--
	dsc.completedSubtasks++

	// Update agent stats
	if stat.agentID > 0 {
		if agentStat, ok := dsc.agentStats[stat.agentID]; ok {
			agentStat.downloadCount++
			agentStat.totalDownloadTime += now.Sub(stat.startTime)
			agentStat.currentSubtaskID = -1
		}
	}

	// Update task status
	completedCount := dsc.completedSubtasks
	runningCount := dsc.runningSubtasks
	dsc.mtx.Unlock()
	dsc.task.updateAndSendTaskStatus("DownloadedParts", completedCount, "DownloadingParts", runningCount)
	dsc.mtx.Lock()
}

// GetOverallStats returns overall download statistics
func (dsc *downloadProgressCollector) getOverallStats() (totalBytes int64, speedBPS int, elapsedTime time.Duration, estimatedRemaining time.Duration) {
	dsc.mtx.Lock()
	defer dsc.mtx.Unlock()

	totalBytes = dsc.totalDownloadedBytes
	speedBPS = dsc.currentSpeedBPS
	elapsedTime = time.Since(dsc.taskStartTime)

	// Calculate estimated remaining time
	if speedBPS > 0 && dsc.task != nil {
		remainingBytes := int64(dsc.task.info.Size) - totalBytes
		if remainingBytes > 0 {
			estimatedRemaining = time.Duration(remainingBytes/int64(speedBPS)) * time.Second
		}
	}

	return
}

// GetAgentStats returns statistics for a specific agent
func (dsc *downloadProgressCollector) getAgentStats(agentID int) (downloadedBytes int64, downloadCount int, avgSpeedBPS int) {
	dsc.mtx.Lock()
	defer dsc.mtx.Unlock()

	stat, exists := dsc.agentStats[agentID]
	if !exists {
		return 0, 0, 0
	}

	downloadedBytes = stat.totalDownloadedBytes
	downloadCount = stat.downloadCount

	if stat.totalDownloadTime > 0 {
		avgSpeedBPS = int(stat.totalDownloadedBytes / int64(stat.totalDownloadTime.Seconds()))
	}

	return
}

// GetSubtaskStats returns statistics for a specific subtask
func (dsc *downloadProgressCollector) getSubtaskStats(subtaskID int) (downloadedBytes int64, isCompleted bool) {
	dsc.mtx.Lock()
	defer dsc.mtx.Unlock()

	stat, exists := dsc.subtaskStats[subtaskID]
	if !exists {
		return 0, false
	}

	return stat.downloadedBytes, stat.isCompleted
}

// calculateSpeedLocked calculates the current download speed using a sliding window
// Must be called with mtx held
func (dsc *downloadProgressCollector) calculateSpeedLocked(now time.Time) {
	cutoff := now.Add(-dsc.speedWindowSize)

	// Remove old samples outside the window
	validIndex := 0
	for i, sample := range dsc.speedSamples {
		if sample.timestamp.After(cutoff) {
			validIndex = i
			break
		}
	}
	if validIndex > 0 {
		dsc.speedSamples = dsc.speedSamples[validIndex:]
	}

	// Calculate total bytes in the window
	var totalBytes int64
	for _, sample := range dsc.speedSamples {
		totalBytes += sample.bytes
	}

	// Calculate speed (bytes per second)
	if len(dsc.speedSamples) > 0 {
		windowDuration := now.Sub(dsc.speedSamples[0].timestamp)
		if windowDuration > 0 {
			dsc.currentSpeedBPS = int(float64(totalBytes) / windowDuration.Seconds())
		}
	} else {
		dsc.currentSpeedBPS = 0
	}

	slog.Debug("Speed calculated",
		"speedBPS", dsc.currentSpeedBPS,
		"samples", len(dsc.speedSamples),
		"windowBytes", totalBytes)
}

// updateTaskStatusLocked updates the task status with speed and calculated fields
// Must be called with mtx held
func (dsc *downloadProgressCollector) updateTaskStatusLocked() {
	if dsc.task == nil {
		return
	}

	speed := dsc.currentSpeedBPS
	totalBytes := dsc.totalDownloadedBytes
	total := len(dsc.task.subtasks)

	// Calculate estimated remaining time
	var estimatedRemaining time.Duration
	if speed > 0 {
		remainingBytes := int64(dsc.task.info.Size) - totalBytes
		if remainingBytes > 0 {
			estimatedRemaining = time.Duration(remainingBytes/int64(speed)) * time.Second
		}
	}

	slog.Debug("Updating task status",
		"speedBPS", speed,
		"totalBytes", totalBytes,
		"totalParts", total,
		"runningParts", dsc.runningSubtasks,
		"completedParts", dsc.completedSubtasks,
		"estimatedRemaining", estimatedRemaining)

	// Release our lock before updating task
	dsc.mtx.Unlock()

	dsc.task.updateAndSendTaskStatus(
		"DownloadSpeed", speed,
		"TotalDownloadedBytes", totalBytes,
		"EstimatedRemainingTime", estimatedRemaining,
		"TotalParts", total)

	// Re-acquire our lock
	dsc.mtx.Lock()
}

// getOrCreateAgentStatLocked gets or creates agent stats
// Must be called with mtx held
func (dsc *downloadProgressCollector) getOrCreateAgentStatLocked(agentID int) *agentStat {
	stat, exists := dsc.agentStats[agentID]
	if !exists {
		stat = &agentStat{
			agentID:          agentID,
			currentSubtaskID: -1,
		}
		dsc.agentStats[agentID] = stat
	}
	return stat
}
