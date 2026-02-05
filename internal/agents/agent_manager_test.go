package agents

import (
	"errors"
	"testing"
	"time"
)

func TestAgentManagerRegisterAndCount(t *testing.T) {
	am := NewAgentManagerWithDefaultConfig()
	defer am.Stop()

	id, err := am.RegisterAgent("endpoint-1", "agent-1", "v1")
	if err != nil {
		t.Fatalf("RegisterAgent error: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}
	if am.GetAgentCount() != 1 {
		t.Fatalf("expected count=1, got %d", am.GetAgentCount())
	}

	if !am.HeartbeatReceived(id) {
		t.Fatalf("expected heartbeat for known agent")
	}
	if am.HeartbeatReceived(9999) {
		t.Fatalf("expected heartbeat for unknown agent to be false")
	}
}

func TestAgentManagerDuplicateRegister(t *testing.T) {
	am := NewAgentManagerWithDefaultConfig()
	defer am.Stop()

	id, err := am.RegisterAgent("endpoint-dup", "agent-dup", "v1")
	if err != nil {
		t.Fatalf("RegisterAgent error: %v", err)
	}

	_, err = am.RegisterAgent("endpoint-dup", "agent-dup", "v1")
	if err == nil {
		t.Fatalf("expected error on duplicate register")
	}
	var existsErr *AlreadyExistsError
	if !errors.As(err, &existsErr) {
		t.Fatalf("expected AlreadyExistsError, got %T", err)
	}
	if existsErr.ID != id {
		t.Fatalf("expected duplicate id %d, got %d", id, existsErr.ID)
	}
}

func TestAgentManagerGetIdleAndRelease(t *testing.T) {
	am := NewAgentManagerWithDefaultConfig()
	defer am.Stop()

	_, err := am.RegisterAgent("endpoint-idle", "agent-idle", "v1")
	if err != nil {
		t.Fatalf("RegisterAgent error: %v", err)
	}

	result := make(chan *AgentInfo, 1)
	abort := make(chan struct{})
	go func() {
		result <- am.GetIdleAgent(abort)
	}()

	var agent *AgentInfo
	select {
	case agent = <-result:
	case <-time.After(2 * time.Second):
		close(abort)
		t.Fatalf("timed out waiting for idle agent")
	}
	if agent == nil {
		t.Fatalf("expected non-nil agent")
	}

	am.ReleaseAgent(agent, true)

	result2 := make(chan *AgentInfo, 1)
	go func() {
		result2 <- am.GetIdleAgent(abort)
	}()

	select {
	case agent = <-result2:
	case <-time.After(2 * time.Second):
		close(abort)
		t.Fatalf("timed out waiting for idle agent after release")
	}
	if agent == nil {
		t.Fatalf("expected non-nil agent after release")
	}
}

func TestAgentManagerDropAndBan(t *testing.T) {
	am := NewAgentManagerWithDefaultConfig()
	defer am.Stop()

	id, err := am.RegisterAgent("endpoint-ban", "agent-ban", "v1")
	if err != nil {
		t.Fatalf("RegisterAgent error: %v", err)
	}

	am.DropAndBanAgent(id, time.Second, "test")
	if am.GetAgentCount() != 0 {
		t.Fatalf("expected count=0 after ban, got %d", am.GetAgentCount())
	}

	_, err = am.RegisterAgent("endpoint-ban", "agent-ban", "v1")
	if err == nil {
		t.Fatalf("expected error when registering banned agent")
	}
	var bannedErr *AgentIsBannedError
	if !errors.As(err, &bannedErr) {
		t.Fatalf("expected AgentIsBannedError, got %T", err)
	}
}
