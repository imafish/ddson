package agents

import "time"

type AgentManager interface {
	RegisterAgent(endpoint string, name string, version string) (int, error)
	HeartbeatReceived(agentID int) bool
	GetIdleAgent(abortChan <-chan struct{}) *AgentInfo
	ReleaseAgent(agent *AgentInfo, success bool)
	DropAndBanAgent(agentID int, duration time.Duration, reason string)
	GetAgentCount() int
	Stop()
}
