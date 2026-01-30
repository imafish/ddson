package agents

type AgentManagerConfig struct {
	HeartbeatCheckIntervalSec      int
	HeartbeatTimeoutSec            int
	MaxConsecutiveErrors           int
	HeartbeatBanDurationSec        int
	ConsecutiveErrorBanDurationSec int
}

func NewDefaultAgentManagerConfig() *AgentManagerConfig {
	return &AgentManagerConfig{
		HeartbeatCheckIntervalSec:      10,
		HeartbeatTimeoutSec:            40,
		MaxConsecutiveErrors:           5,
		HeartbeatBanDurationSec:        60,
		ConsecutiveErrorBanDurationSec: 600,
	}
}
