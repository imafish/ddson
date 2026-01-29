package agents

import (
	"time"
)

type AgentStatistics struct {
	TotalTasksRun     int
	TotalErrors       int
	ConsecutiveErrors int
	ConnectedAt       int64 // Unix timestamp
}

func (as *AgentStatistics) GetUptime() time.Duration {
	return time.Duration(time.Now().Unix()-as.ConnectedAt) * time.Second
}

func newAgentStatistics() AgentStatistics {
	return AgentStatistics{
		TotalTasksRun:     0,
		TotalErrors:       0,
		ConsecutiveErrors: 0,
		ConnectedAt:       time.Now().Unix(),
	}
}

type AgentStatus int

const (
	AgentStatusIdle AgentStatus = iota
	AgentStatusQueued
	AgentStatusBusy
	AgentStatusBanned
)

type AgentInfo struct {
	name       string
	id         int
	version    string
	endpoint   string
	status     AgentStatus
	statistics AgentStatistics
}

func NewAgentInfo(name string, id int, version string, endpoint string) *AgentInfo {
	return &AgentInfo{
		name:       name,
		id:         id,
		version:    version,
		endpoint:   endpoint,
		statistics: newAgentStatistics(),
	}
}

func (ai *AgentInfo) GetName() string {
	return ai.name
}
func (ai *AgentInfo) GetID() int {
	return ai.id
}
func (ai *AgentInfo) GetVersion() string {
	return ai.version
}
func (ai *AgentInfo) GetAddr() string {
	return ai.endpoint
}
func (ai *AgentInfo) GetStatus() AgentStatus {
	return ai.status
}
func (ai *AgentInfo) GetStatistics() AgentStatistics {
	return ai.statistics
}
