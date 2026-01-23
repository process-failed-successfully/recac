package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
    "strings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"recac/internal/db"
	"recac/internal/notify"
	"recac/internal/telemetry"
)

// TestifyMockAgent is a generic mock for agent.Agent
type TestifyMockAgent struct {
	mock.Mock
}

func (m *TestifyMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *TestifyMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	resp := args.String(0)
	if resp != "" && onChunk != nil {
		onChunk(resp)
	}
	return resp, args.Error(1)
}

func TestProcessResponse_BashExecution(t *testing.T) {
	// Setup
	workspace := t.TempDir()
	mockDocker := &MockDockerClient{}

	session := &Session{
		Workspace:     workspace,
		Docker:        mockDocker,
		Logger:        telemetry.NewLogger(true, "", false),
		Project:       "test-project",
		UseLocalAgent: false,
		ContainerID:   "test-container",
	}

	// Mock Docker Exec
	mockDocker.ExecFunc = func(ctx context.Context, containerID string, cmd []string) (string, error) {
		// Verify command structure
		if len(cmd) > 2 && cmd[0] == "/bin/bash" && cmd[1] == "-c" {
			script := cmd[2]
			if strings.Contains(script, "echo hello") {
				return "hello\n", nil
			}
		}
		return "", nil
	}

	// Test case: Valid bash block
	response := "Here is the code:\n```bash\necho hello\n```\nDone."
	output, err := session.ProcessResponse(context.Background(), response)
	require.NoError(t, err)
	assert.Contains(t, output, "Command Output:")
	assert.Contains(t, output, "hello")
}

func TestProcessResponse_JSONSkip(t *testing.T) {
	// Setup
	session := &Session{
		Logger: telemetry.NewLogger(true, "", false),
	}

	// Test case: JSON block mistaken for bash
	response := "Data:\n```bash\n{\"key\": \"value\"}\n```"
	output, err := session.ProcessResponse(context.Background(), response)
	require.NoError(t, err)
	assert.Contains(t, output, "Skipped JSON Block")
}

func TestProcessResponse_Blocker_DB(t *testing.T) {
	// Setup
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, err := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	require.NoError(t, err)
	defer store.Close()

	session := &Session{
		Workspace: workspace,
		DBStore:   store,
		Project:   "test-project",
		Logger:    telemetry.NewLogger(true, "", false),
	}

	// Set blocker signal
	err = store.SetSignal("test-project", "BLOCKER", "Something is wrong")
	require.NoError(t, err)

	// Test execution
	_, err = session.ProcessResponse(context.Background(), "some response")
	assert.Error(t, err)
	assert.Equal(t, ErrBlocker, err)
}

func TestProcessResponse_Blocker_LegacyFile(t *testing.T) {
	// Setup
	workspace := t.TempDir()
	mockDocker := &MockDockerClient{}

	session := &Session{
		Workspace:     workspace,
		Docker:        mockDocker,
		Project:       "test-project",
		Logger:        telemetry.NewLogger(true, "", false),
		ContainerID:   "test-container",
	}

	// Mock Docker Exec to simulate finding a blocker file
	mockDocker.ExecFunc = func(ctx context.Context, containerID string, cmd []string) (string, error) {
		cmdStr := strings.Join(cmd, " ")
		if strings.Contains(cmdStr, "blockers.txt") {
			return "There is a blocker", nil
		}
		return "", nil
	}

	// Test execution
	_, err := session.ProcessResponse(context.Background(), "some response")
	assert.Error(t, err)
	assert.Equal(t, ErrBlocker, err)
}

func TestRunManagerAgent(t *testing.T) {
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, err := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	require.NoError(t, err)
	defer store.Close()

	mockManager := new(TestifyMockAgent)

	session := &Session{
		Workspace:    workspace,
		Project:      "test-project",
		DBStore:      store,
		ManagerAgent: mockManager,
		Logger:       telemetry.NewLogger(true, "", false),
		Notifier:     notify.NewManager(func(string, ...interface{}) {}),
	}

	mockDocker := &MockDockerClient{}
	session.Docker = mockDocker
	session.ContainerID = "test-container"

	// 1. Test Manager Approval (via Signal)
	// We simulate the agent running a command that updates the DB (simulating a tool call)
	mockDocker.ExecFunc = func(ctx context.Context, id string, cmd []string) (string, error) {
		cmdStr := strings.Join(cmd, " ")
		if strings.Contains(cmdStr, "sign-off") {
			store.SetSignal("test-project", "PROJECT_SIGNED_OFF", "true")
			return "Signed off", nil
		}
		return "", nil
	}

	mockManager.On("Send", mock.Anything, mock.Anything).Return("Approve\n```bash\nrecac-tool sign-off\n```", nil).Once()

	err = session.runManagerAgent(context.Background())
	assert.NoError(t, err)

	// 2. Test Manager Rejection
	mockManager.On("Send", mock.Anything, mock.Anything).Return("Reject", nil).Once()

	// Clear signal first
	store.DeleteSignal("test-project", "PROJECT_SIGNED_OFF")
	os.Remove(filepath.Join(workspace, "PROJECT_SIGNED_OFF"))

	err = session.runManagerAgent(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manager review did not result in sign-off")
}

func TestRunLoop_Initialization(t *testing.T) {
	workspace := t.TempDir()

	// Create app_spec.txt
	os.WriteFile(filepath.Join(workspace, "app_spec.txt"), []byte("spec"), 0644)

	mockDocker := &MockDockerClient{}
	mockAgent := new(TestifyMockAgent)

	session := &Session{
		Workspace:        workspace,
		Docker:           mockDocker,
		Agent:            mockAgent,
		Logger:           telemetry.NewLogger(true, "", false),
		MaxIterations:    1,
		ManagerFrequency: 5,
		Project:          "test-project",
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
	}

	// Mocks for LoadAgentState (none)

	// Mocks for SelectPrompt
	// It will try to load features.
	// If no features, it selects Initializer.
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("Plan: do nothing", nil).Once()

	err := session.RunLoop(context.Background())
	assert.Equal(t, ErrMaxIterations, err) // Should run 1 iteration and hit max

	mockAgent.AssertExpectations(t)
}

func TestRunLoop_Success(t *testing.T) {
	workspace := t.TempDir()

    // Setup app_spec.txt
    os.WriteFile(filepath.Join(workspace, "app_spec.txt"), []byte("spec"), 0644)

	// Setup feature_list.json to ensure we don't trigger Initializer
    featureContent := `{"features": [{"id": "1", "description": "task 1", "status": "todo"}]}`
    os.WriteFile(filepath.Join(workspace, "feature_list.json"), []byte(featureContent), 0644)

	mockDocker := &MockDockerClient{}
	mockAgent := new(TestifyMockAgent)

	session := &Session{
		Workspace:        workspace,
		Docker:           mockDocker,
		Agent:            mockAgent,
		Logger:           telemetry.NewLogger(true, "", false),
		MaxIterations:    1, // Run 1 iteration then exit
		ManagerFrequency: 5,
		Project:          "test-project",
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
	}

    // Mock Agent Response
    // Agent should return a command
    mockAgent.On("Send", mock.Anything, mock.Anything).Return("Here is code:\n```bash\necho success\n```", nil).Once()

    // Mock Docker Exec
    mockDocker.ExecFunc = func(ctx context.Context, id string, cmd []string) (string, error) {
        cmdStr := strings.Join(cmd, " ")
        if strings.Contains(cmdStr, "recac_blockers.txt") || strings.Contains(cmdStr, "blockers.txt") {
            return "", nil // No blockers found
        }
        return "success", nil
    }

	err := session.RunLoop(context.Background())
	assert.Equal(t, ErrMaxIterations, err)

    mockAgent.AssertExpectations(t)
}
