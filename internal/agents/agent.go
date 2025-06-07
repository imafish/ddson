package agents

import "log/slog"

type AgentInfo struct {
	name    string
	id      int
	version string
	addr    string
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
	return ai.addr
}

type Agent interface {
	Close()
	GetAgentInfo() *AgentInfo // returns the agent info

	RunTask(func(*AgentInfo) error)

	GetErrorCount() int // returns the error count of the agent
	Retire()            // marks the agent as retired, meaning it will not accept new tasks any more

	setID(id int) // sets the ID of the agent, used internally
}

type AgentImpl struct {
	agentInfo  *AgentInfo // contains the agent's information
	errorCount int        // number of errors encountered by the agent
}

func NewAgent(name string, version string, addr string) *AgentImpl {
	return &AgentImpl{
		agentInfo: &AgentInfo{
			name:    name,
			id:      -1,
			version: version,
			addr:    addr,
		},
		errorCount: 0,
	}
}

func (a *AgentImpl) Close() {
	// Implement any cleanup logic if necessary
}

func (a *AgentImpl) GetAgentInfo() *AgentInfo {
	return a.agentInfo
}

func (a *AgentImpl) RunTask(taskFunc func(agentInfo *AgentInfo) error) {
	if err := taskFunc(a.GetAgentInfo()); err != nil {
		a.errorCount++ // Increment error count if the task fails
		// if the agent has encountered too many errors, retire it
		if a.errorCount > 3 {
			a.Retire()
		}
	} else if a.errorCount > 0 {
		a.errorCount--
	}
}

func (a *AgentImpl) Retire() {
	// Implement logic to mark the agent as retired
	// This could involve setting a flag or removing it from a list of active agents
	// For now, we will just log the retirement
	slog.Info("Agent retired", "agentID", a.agentInfo.id, "agentName", a.agentInfo.name)
}

func (a *AgentImpl) GetErrorCount() int {
	return a.errorCount
}

func (a *AgentImpl) setID(id int) {
	a.agentInfo.id = id
	slog.Debug("Agent ID set", "agentName", a.agentInfo.name, "agentID", id)
}
