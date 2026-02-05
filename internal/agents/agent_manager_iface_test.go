package agents

import "testing"

func TestAgentManagerImplementsInterface(t *testing.T) {
	var _ AgentManager = (*AgentManagerImpl)(nil)
}
