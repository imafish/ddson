package agents

import (
	"fmt"
	"time"
)

// AlreadyExistsError is returned when an agent with the same ID already exists in the agent list.
type AlreadyExistsError struct {
	ID int // ID of the agent that already exists
}

// Error implements the error interface for AlreadyExistsError.
func (e *AlreadyExistsError) Error() string {
	return fmt.Sprintf("agent with ID %d already exists", e.ID)
}

// AgentIsBannedError is returned when an agent is banned and cannot be added to the agent list.
type AgentIsBannedError struct {
	AgentAddr string
	Until     time.Time
}

// Error implements the error interface for AgentIsBannedError.
func (e *AgentIsBannedError) Error() string {
	return "agent is banned: " + e.AgentAddr + " until " + e.Until.String()
}
