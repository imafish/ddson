package agents

import (
	"sync"
	"time"

	"golang.org/x/exp/slog"
)

type AgentManager struct {
	config *AgentManagerConfig

	mtx             sync.RWMutex
	agents          map[int]*AgentInfo   // agentID -> AgentInfo
	bannedAgents    map[string]time.Time // agentEndpoint -> banUntil
	agentHeartbeats map[int]time.Time    // agentID -> lastHeartbeatTime
	nextAgentID     int

	idleAgentChan chan *AgentInfo
	stopFlag      chan struct{}
	wg            sync.WaitGroup
}

func NewAgentManager(config *AgentManagerConfig) *AgentManager {
	newAgentManager := &AgentManager{
		config:        config,
		agents:        make(map[int]*AgentInfo),
		bannedAgents:  make(map[string]time.Time),
		idleAgentChan: make(chan *AgentInfo, 100),
		stopFlag:      make(chan struct{}),
		nextAgentID:   1,
	}

	go newAgentManager.monitorHeartbeats()
	return newAgentManager
}

func NewAgentManagerWithDefaultConfig() *AgentManager {
	return NewAgentManager(NewDefaultAgentManagerConfig())
}

func (am *AgentManager) RegisterAgent(endpoint string, name string, version string) (int, error) {
	am.mtx.Lock()
	defer am.mtx.Unlock()

	// if already exists, ignore
	agentID := -1
	for _, agent := range am.agents {
		if agent.endpoint == endpoint {
			agentID = agent.id
			break
		}
	}
	if agentID != -1 {
		slog.Info("Agent already registered, ignoring", "endpoint", endpoint, "agentID", agentID)
		return 0, &AlreadyExistsError{ID: agentID, Endpoint: endpoint}
	}

	// if banned, ignore
	if banUntil, banned := am.bannedAgents[endpoint]; banned {
		slog.Info("Agent is banned, ignoring", "agentID", agentID, "banUntil", banUntil)
		return 0, &AgentIsBannedError{Endpoint: endpoint, Until: banUntil}
	}

	// register new agent
	agent := &AgentInfo{
		id:       am.nextAgentID,
		endpoint: endpoint,
		name:     name,
		version:  version,
		status:   AgentStatusIdle,
	}
	am.agents[agent.id] = agent
	am.agentHeartbeats[agent.id] = time.Now()
	am.nextAgentID++

	return agent.id, nil
}

func (am *AgentManager) HeartbeatReceived(agentID int) {
	am.mtx.Lock()
	defer am.mtx.Unlock()
	am.agentHeartbeats[agentID] = time.Now()
}

func (am *AgentManager) DropAndBanAgent(agentID int, duration time.Duration, reason string) {
	am.mtx.Lock()
	defer am.mtx.Unlock()

	agent, exists := am.agents[agentID]
	if !exists {
		return
	}

	agent.status = AgentStatusBanned
	am.bannedAgents[agent.endpoint] = time.Now().Add(duration)
	delete(am.agents, agentID)
	delete(am.agentHeartbeats, agentID)
}

func (am *AgentManager) Stop() {
	slog.Info("Stopping AgentManager")

	close(am.stopFlag)
	close(am.idleAgentChan)
	am.wg.Wait()
}

func (am *AgentManager) GetIdleAgentChan() <-chan *AgentInfo {
	return am.idleAgentChan
}

func (am *AgentManager) ReleaseAgent(agent *AgentInfo, successful bool) {
	am.mtx.Lock()
	defer am.mtx.Unlock()

	// check if agent is still registered and not banned
	_, exists := am.agents[agent.id]
	if !exists {
		slog.Warn("Agent is no longer registered, ignoring release", "agentID", agent.id)
		return
	}
	if banUntil, banned := am.bannedAgents[agent.endpoint]; banned {
		slog.Warn("Agent is banned, ignoring release", "agentID", agent.id, "banUntil", banUntil)
		return
	}

	// update agent statistics
	agent.statistics.TotalTasksRun++
	if successful {
		agent.statistics.ConsecutiveErrors = 0
	} else {
		agent.statistics.ConsecutiveErrors++
		agent.statistics.TotalErrors++
		if agent.statistics.ConsecutiveErrors >= am.config.MaxConsecutiveErrors {
			slog.Info("Agent has too many consecutive errors, dropping and banning", "agentID", agent.id)
			am.DropAndBanAgent(agent.id, time.Duration(am.config.ConsecutiveErrorBanDurationSec)*time.Second, "too many consecutive errors")
			return
		}
	}

	// try putting the agent back to idle channel
	select {
	case am.idleAgentChan <- agent:
		agent.status = AgentStatusQueued
		am.scanAndQueueAgentsLocked()

	default:
		// the channel is full, mark the agent as idle
		agent.status = AgentStatusIdle
		slog.Warn("Idle agent channel is full, marking agent as idle without re-adding to channel", "agentID", agent.id)
	}
}

func (am *AgentManager) GetAgentCount() int {
	am.mtx.RLock()
	defer am.mtx.RUnlock()
	return len(am.agents)
}

/*
 * private methods
 */
func (am *AgentManager) monitorHeartbeats() {
	ticker := time.NewTicker(time.Duration(am.config.HeartbeatCheckIntervalSec) * time.Second)
	defer ticker.Stop()
	defer am.wg.Done()
	slog.Info("Started agent heartbeat monitor")

	for {
		select {
		case <-ticker.C:
			am.checkHeartbeats()
		case <-am.stopFlag:
			return
		}
	}
}

func (am *AgentManager) checkHeartbeats() {
	am.mtx.RLock()
	defer am.mtx.RUnlock()

	slog.Debug("Checking agent heartbeats", "agentCount", len(am.agentHeartbeats))
	for agentID, lastHeartbeat := range am.agentHeartbeats {
		slog.Debug("Agent heartbeat check", "agentID", agentID, "lastHeartbeat", lastHeartbeat)

		if time.Since(lastHeartbeat) > time.Duration(am.config.HeartbeatTimeoutSec)*time.Second {
			slog.Info("Agent heartbeat timeout, dropping and banning agent", "agentID", agentID)
			am.DropAndBanAgent(agentID, time.Duration(am.config.HeartbeatBanDurationSec)*time.Second, "heartbeat timeout")
		}
	}
}

func (am *AgentManager) scanAndQueueAgentsLocked() {
	for _, agent := range am.agents {
		if agent.status == AgentStatusIdle {
			select {
			case am.idleAgentChan <- agent:
				slog.Debug("Queued idle agent", "agentID", agent.id)
				agent.status = AgentStatusQueued
			default:
				// channel full, stop trying
				return
			}
		}
	}
}
