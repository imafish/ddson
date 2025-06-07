package agents

import (
	"log/slog"
	"sync"
	"time"
)

type AgentList interface {
	AddAgent(agent Agent) (int, error)
	RemoveAgent(id int)
	GetAgentByID(id int) Agent
	Count() int

	RunTask(task func(*AgentInfo) error) error // runs a task on the agent list, blocking until a free agent is available

	BanAgent(id int, reason string, until time.Time) // marks an agent as banned, preventing it from accepting new tasks until the specified time
}

type AgentListImpl struct {
	freeAgents   map[int]Agent
	busyAgents   map[int]Agent
	bannedAgents map[string]time.Time // map to track banned agents by their address

	nextID int
	mtx    sync.Mutex // mutex to protect the agents map
	cond   *sync.Cond // condition variable to signal when an agent is available
}

func NewAgentList() *AgentListImpl {
	agentList := &AgentListImpl{
		freeAgents:   make(map[int]Agent),
		busyAgents:   make(map[int]Agent),
		bannedAgents: make(map[string]time.Time),
	}
	agentList.cond = sync.NewCond(&agentList.mtx)
	return agentList
}

func (al *AgentListImpl) AddAgent(agent Agent) (int, error) {
	al.mtx.Lock()
	defer al.mtx.Unlock()

	agentAddr := agent.GetAgentInfo().GetAddr()

	isBanned, until := al.isAgentBanned(agentAddr)
	if isBanned {
		return 0, &AgentIsBannedError{
			AgentAddr: agentAddr,
			Until:     until,
		}
	}

	id := al.nextID
	al.nextID++
	agent.setID(id)

	al.freeAgents[id] = agent
	al.cond.Signal() // Signal that a new agent has been added
	return id, nil
}

func (al *AgentListImpl) RemoveAgent(id int) {
	al.mtx.Lock()
	defer al.mtx.Unlock()

	if _, exists := al.freeAgents[id]; exists {
		delete(al.freeAgents, id)
	} else if _, busyExists := al.busyAgents[id]; busyExists {
		delete(al.busyAgents, id)
	} else {
		return // Agent with this ID does not exist
	}
}

func (al *AgentListImpl) GetAgentByID(id int) Agent {
	al.mtx.Lock()
	defer al.mtx.Unlock()

	agent, exists := al.freeAgents[id]
	if exists {
		return agent
	}
	agent, busyExists := al.busyAgents[id]
	if busyExists {
		return agent
	}

	return nil
}

func (al *AgentListImpl) Count() int {
	al.mtx.Lock()
	defer al.mtx.Unlock()

	return len(al.freeAgents) + len(al.busyAgents)
}

func (al *AgentListImpl) BanAgent(id int, reason string, until time.Time) {
	al.mtx.Lock()
	defer al.mtx.Unlock()

	agent := al.GetAgentByID(id)
	if agent == nil {
		slog.Warn("Attempted to ban non-existent agent", "id", id, "reason", reason)
		return
	}

	if until.IsZero() {
		slog.Warn("Attempted to ban agent without a valid until time", "id", id, "reason", reason)
		return
	}
	agentAddr := agent.GetAgentInfo().GetAddr()
	al.bannedAgents[agentAddr] = until // Ban the agent by its address
	slog.Info("Banned agent", "id", id, "address", agentAddr, "reason", reason, "until", until)

	// Remove the agent from free and busy lists if it exists
	al.RemoveAgent(id)
}

func (al *AgentListImpl) RunTask(task func(*AgentInfo) error) error {
	var err error
	for i := 0; i < 3; i++ {
		err = al.runTaskOnce(task)

		if err == nil {
			slog.Info("Task executed successfully on agent", "attempt", i+1)
			return nil
		}
	}

	slog.Error("Failed to execute task after retries", "error", err)
	return err
}

func (al *AgentListImpl) getOneFreeAgent() Agent {
	al.mtx.Lock()
	defer al.mtx.Unlock()

	for len(al.freeAgents) == 0 {
		al.cond.Wait() // Wait until a free agent is available
	}
	for id, agent := range al.freeAgents {
		delete(al.freeAgents, id) // Remove from free agents
		al.busyAgents[id] = agent // Add to busy agents
		return agent              // Return the first available free agent
	}

	return nil // No free agents available
}

func (al *AgentListImpl) freeAgent(id int) {
	al.mtx.Lock()
	defer al.mtx.Unlock()

	if agent, exists := al.busyAgents[id]; exists {
		delete(al.busyAgents, id) // Remove from busy agents
		al.freeAgents[id] = agent // Add to free agents
		al.cond.Signal()          // Signal that an agent has been freed
	}
}

func (al *AgentListImpl) isAgentBanned(addr string) (bool, time.Time) {
	al.mtx.Lock()
	defer al.mtx.Unlock()

	until, exists := al.bannedAgents[addr]
	if !exists {
		return false, time.Time{}
	}

	if time.Now().After(until) {
		delete(al.bannedAgents, addr) // Remove the ban if the time has passed
		return false, time.Time{}
	}

	return true, until // Agent is still banned
}

func (al *AgentListImpl) runTaskOnce(task func(*AgentInfo) error) error {
	agent := al.getOneFreeAgent()
	agentInfo := agent.GetAgentInfo()
	agentID := agentInfo.GetID()
	defer al.freeAgent(agentID)
	err := task(agentInfo)

	if err != nil {
		if agent.GetErrorCount() > 3 {
			slog.Warn("Agent encountered too many errors, retiring", "agentID", agentID, "errorCount", agent.GetErrorCount())
			agent.Retire()                                                                        // Retire the agent if it has too many errors
			al.BanAgent(agentID, "Retired due to too many errors", time.Now().Add(5*time.Minute)) // Ban for 5 minutes

			// TODO: maybe we don't remove the agent. just BAN, and prevent if from accepting new tasks.
		}
	}

	return err
}
