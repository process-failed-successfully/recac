package runner

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/docker"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"testing"
	"time"
)

// MockLoopDocker implements DockerClient interface
type MockLoopDocker struct {
	CheckDaemonFunc   func(ctx context.Context) error
	RunContainerFunc  func(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error)
	StopContainerFunc func(ctx context.Context, containerID string) error
	ExecFunc          func(ctx context.Context, containerID string, cmd []string) (string, error)
}

func (m *MockLoopDocker) CheckDaemon(ctx context.Context) error {
	if m.CheckDaemonFunc != nil {
		return m.CheckDaemonFunc(ctx)
	}
	return nil
}

func (m *MockLoopDocker) RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error) {
	if m.RunContainerFunc != nil {
		return m.RunContainerFunc(ctx, imageRef, workspace, extraBinds, env, user)
	}
	return "mock-container", nil
}

func (m *MockLoopDocker) StopContainer(ctx context.Context, containerID string) error {
	if m.StopContainerFunc != nil {
		return m.StopContainerFunc(ctx, containerID)
	}
	return nil
}

func (m *MockLoopDocker) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, containerID, cmd)
	}
	return "", nil
}

func (m *MockLoopDocker) ExecAsUser(ctx context.Context, containerID string, user string, cmd []string) (string, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, containerID, cmd)
	}
	return "", nil
}

func (m *MockLoopDocker) ImageExists(ctx context.Context, tag string) (bool, error) {
	return true, nil
}

func (m *MockLoopDocker) ImageBuild(ctx context.Context, opts docker.ImageBuildOptions) (string, error) {
	return opts.Tag, nil
}

func (m *MockLoopDocker) PullImage(ctx context.Context, imageRef string) error {
	return nil
}

// MockLoopAgent implements Agent interface
type MockLoopAgent struct {
	Response  string
	Responses []string
	CallCount int
}

func (m *MockLoopAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.CallCount < len(m.Responses) {
		resp := m.Responses[m.CallCount]
		m.CallCount++
		return resp, nil
	}
	return m.Response, nil
}

func (m *MockLoopAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, _ := m.Send(ctx, prompt)
	if onChunk != nil {
		onChunk(resp)
	}
	return resp, nil
}

func TestSession_RunLoop_Success(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	mockDocker := &MockLoopDocker{}
	mockManager := &MockLoopAgent{Response: "Approved."}
	mockAgent := &MockLoopAgent{
		Responses: []string{
			"I will do success.\n```bash\necho success\n```",
			"I am done.",
		},
	}
	mockCleaner := &MockLoopAgent{Response: "Cleaned."}
	mockQA := &MockLoopAgent{Response: "PASS"}

	s := &Session{
		Workspace:        tmpDir,
		Docker:           mockDocker,
		ManagerAgent:     mockManager,
		Agent:            mockAgent,
		CleanerAgent:     mockCleaner,
		QAAgent:          mockQA,
		DBStore:          store,
		MaxIterations:    5,
		ManagerFrequency: 10,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
	}

	// Wait, RunLoop will loop until MaxIterations or COMPLETED.
	// My mock agent doesn't set COMPLETED signal.
	// ProcessResponse sets it if specific keyword? No.
	// checkAutoQA sets it if passes.
	// I need to manipulate state or let it run out or error.

	// Let's create a signal manually to stop it after some iterations if needed
	// Or rely on MaxIterations.

	ctx := context.Background()
	err := s.RunLoop(ctx)
	if err != nil && !errors.Is(err, ErrNoOp) {
		t.Errorf("RunLoop failed: %v", err)
	}

	// Verify interactions
	if mockAgent.CallCount < 1 {
		t.Error("Agent should have been called")
	}
}

func TestSession_RunLoop_StalledBreaker(t *testing.T) {
	// Setup logic for stalled breaker...
	// This might duplicate session_circuit_breaker_test.go logic but integrated.
}

func TestRunLoop_CrashWithNilDB_MultiAgent(t *testing.T) {
	// 1. Setup Workspace
	tmpDir := t.TempDir()

	// Ensure app_spec.txt exists (RunLoop requirement)
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	// Ensure feature_list.json exists so SelectPrompt picks CodingAgent and not Initializer
	fl := db.FeatureList{
		Features: []db.Feature{
			{ID: "feat-1", Description: "Feature 1", Status: "pending"},
		},
	}
	data, _ := json.Marshal(fl)
	os.WriteFile(filepath.Join(tmpDir, "feature_list.json"), data, 0644)

	// 2. Initialize Session with NIL DBStore and MaxAgents > 1
	mockAgent := &MockLoopAgent{
		Responses: []string{"I am a coding agent."},
	}

	s := &Session{
		Workspace:        tmpDir,
		Docker:           &MockLoopDocker{}, // Mock Docker to avoid restricted mode logs/checks
		Agent:            mockAgent,
		DBStore:          nil, // THE CAUSE OF CRASH
		MaxIterations:    1,   // Only run once
		ManagerFrequency: 100, // Avoid Manager
		MaxAgents:        2,   // Enable Multi-Agent Logic
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		SleepFunc:        func(d time.Duration) {}, // No sleep
	}

	// 3. Run Loop
	// This should NOT panic. If it panics, the test fails (Go test catches panic).
	err := s.RunLoop(context.Background())

	// We verify that it ran without panic.
	// Ideally, it should log a warning and fallback to single agent, completing iteration 1.
	// Since MaxIterations is 1, it will return ErrMaxIterations, which is expected.
	if err != nil && err != ErrMaxIterations {
		t.Errorf("Unexpected error: %v", err)
	}
}
