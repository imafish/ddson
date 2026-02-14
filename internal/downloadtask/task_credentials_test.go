package downloadtask

import (
	"testing"
	"time"

	"github.com/imafish/ddson/internal/agents"
)

// mockAgentManager is a simple mock for testing
type mockAgentManager struct{}

func (m *mockAgentManager) RegisterAgent(endpoint string, name string, version string) (int, error) {
	return 0, nil
}

func (m *mockAgentManager) HeartbeatReceived(agentID int) bool {
	return true
}

func (m *mockAgentManager) GetIdleAgent(abortChan <-chan struct{}) *agents.AgentInfo {
	return nil
}

func (m *mockAgentManager) ReleaseAgent(agent *agents.AgentInfo, success bool) {}

func (m *mockAgentManager) DropAndBanAgent(agentID int, duration time.Duration, reason string) {}

func (m *mockAgentManager) GetAgentCount() int {
	return 0
}

func (m *mockAgentManager) Stop() {}

func TestNewTaskWithCredentials(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{
			name:     "with credentials",
			username: "testuser",
			password: "testpass",
		},
		{
			name:     "with empty credentials",
			username: "",
			password: "",
		},
		{
			name:     "with only username",
			username: "testuser",
			password: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAgentMgr := &mockAgentManager{}
			task := NewTask(
				"http://example.com/file.txt",
				"checksum123",
				1024,
				nil, // stream
				1,   // taskId
				0,   // idOfClient
				mockAgentMgr,
				tt.username,
				tt.password,
			)

			if task == nil {
				t.Fatal("expected task to be created")
			}

			if task.info.Username != tt.username {
				t.Errorf("expected username=%q, got %q", tt.username, task.info.Username)
			}

			if task.info.Password != tt.password {
				t.Errorf("expected password=%q, got %q", tt.password, task.info.Password)
			}

			// Verify other fields are set correctly
			if task.info.DownloadUrl != "http://example.com/file.txt" {
				t.Errorf("unexpected download URL: %s", task.info.DownloadUrl)
			}

			if task.info.Checksum != "checksum123" {
				t.Errorf("unexpected checksum: %s", task.info.Checksum)
			}

			if task.info.Size != 1024 {
				t.Errorf("unexpected size: %d", task.info.Size)
			}

			if task.info.ID != 1 {
				t.Errorf("unexpected ID: %d", task.info.ID)
			}
		})
	}
}

func TestNewTaskBackwardCompatibility(t *testing.T) {
	// Test that NewTask (without credentials) still works
	mockAgentMgr := &mockAgentManager{}
	task := NewTask(
		"http://example.com/file.txt",
		"checksum123",
		1024,
		nil, // stream
		1,   // taskId
		0,   // idOfClient
		mockAgentMgr,
		"",
		"",
	)

	if task == nil {
		t.Fatal("expected task to be created")
	}

	// Credentials should be empty strings
	if task.info.Username != "" {
		t.Errorf("expected empty username, got %q", task.info.Username)
	}

	if task.info.Password != "" {
		t.Errorf("expected empty password, got %q", task.info.Password)
	}
}

func TestTaskInfoCredentialsStorage(t *testing.T) {
	info := taskInfo{
		ID:          1,
		DownloadUrl: "http://example.com",
		Checksum:    "abc123",
		Size:        2048,
		Username:    "admin",
		Password:    "secret",
	}

	if info.Username != "admin" {
		t.Errorf("expected username=%q, got %q", "admin", info.Username)
	}

	if info.Password != "secret" {
		t.Errorf("expected password=%q, got %q", "secret", info.Password)
	}
}
