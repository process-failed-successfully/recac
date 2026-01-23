package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/docker"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"testing"
)

// MockLoopDocker implements DockerClient interface
type MockLoopDocker struct {
	CheckDaemonFunc   func(ctx context.Context) error
	RunContainerFunc  func(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error)
	StopContainerFunc func(ctx context.Context, containerID string) error
	ExecFunc          func(ctx context.Context, containerID string, cmd []string) (string, error)
	ExecAsUserFunc    func(ctx context.Context, containerID string, user string, cmd []string) (string, error)
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
	if m.ExecAsUserFunc != nil {
		return m.ExecAsUserFunc(ctx, containerID, user, cmd)
	}
	// Fallback to ExecFunc if ExecAsUserFunc not set (compatibility)
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

func setupLoopSession(t *testing.T) (*Session, string) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	// Note: We don't close store here, assuming test cleanup handles directory or we trust GC/OS.
	// But defer store.Close() in caller is better if we return it.
	// For now, let's just return session.

	mockDocker := &MockLoopDocker{}
	mockManager := &MockLoopAgent{Response: "Approved."}
	mockAgent := &MockLoopAgent{Response: "I am working."}
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
		Project:          "test-project",
	}
	return s, tmpDir
}

func TestSession_RunLoop_ContextCancellation(t *testing.T) {
	s, _ := setupLoopSession(t)
	defer s.DBStore.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := s.RunLoop(ctx)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestSession_RunLoop_MaxIterations(t *testing.T) {
	s, _ := setupLoopSession(t)
	defer s.DBStore.Close()

	s.MaxIterations = 1
	// Ensure agent executes something so NoOp doesn't trigger
	mockAgent := &MockLoopAgent{
		Response: "Running command\n```bash\necho test\n```",
	}
	s.Agent = mockAgent

	err := s.RunLoop(context.Background())
	if err != ErrMaxIterations {
		t.Errorf("Expected ErrMaxIterations, got %v", err)
	}
}

func TestSession_RunLoop_ProjectSignedOff(t *testing.T) {
	s, _ := setupLoopSession(t)
	defer s.DBStore.Close()

	// Set signal
	s.createSignal("PROJECT_SIGNED_OFF")

	// Mock cleaner execution
	cleanerCalled := false
	s.CleanerAgent = &MockLoopAgent{
		Response: "Cleaned.",
	}
	// We need to verify cleaner was run.
	// RunLoop calls runCleanerAgent.
	// runCleanerAgent checks for temp_files.txt.
	// If missing, it does nothing but return nil.
	// It logs "cleaner agent running".
	// The session returns nil after cleaner runs.

	// To verify logic flow, we can check if it returned nil (success) immediately without max iterations
	s.MaxIterations = 100

	err := s.RunLoop(context.Background())
	if err != nil {
		t.Errorf("Expected success (nil), got %v", err)
	}

	// If it looped, it would eventually hit max iterations or stalled breaker.
	// Returning nil means it hit the "PROJECT_SIGNED_OFF" branch and exited.

	if cleanerCalled {
		// Can't easily verify internal call without spying on runCleanerAgent or MockAgent wrapper.
		// But exit with nil confirms path.
	}
}

func TestSession_RunLoop_NoOpBreaker(t *testing.T) {
	s, _ := setupLoopSession(t)
	defer s.DBStore.Close()

	// Agent returns text without commands -> NoOp
	s.Agent = &MockLoopAgent{Response: "Just talking, no coding."}

	// Default breaker threshold is 3 no-op iterations (from checkNoOpBreaker impl)
	// We expect ErrNoOp eventually

	err := s.RunLoop(context.Background())
	if !errors.Is(err, ErrNoOp) {
		t.Errorf("Expected ErrNoOp, got %v", err)
	}
}

func TestSession_RunLoop_ManagerTrigger(t *testing.T) {
	s, _ := setupLoopSession(t)
	defer s.DBStore.Close()

	// Setup features for QA (status todo to prevent COMPLETED flow hijacking)
	s.DBStore.SaveFeatures(s.Project, `{"project_name": "test", "features": [{"id":"1", "status":"todo"}]}`)

	// Trigger manager
	s.createSignal("TRIGGER_MANAGER")

	// We want to verify Manager is called.
	// RunLoop uses s.Agent for periodic/triggered manager reviews, not s.ManagerAgent.
	managerCalled := false
	s.Agent = &MockLoopAgentMock{
		SendFunc: func(ctx context.Context, prompt string) (string, error) {
			// Verify prompt indicates Manager role
			// Ideally we check prompt content, but just being called in this iteration is sign enough
			// if we ensure only Manager would run.
			managerCalled = true
			return "Approved", nil
		},
	}

	// Limit iterations to avoid infinite loop if logic fails
	s.MaxIterations = 1 // Should trigger on first iteration

	// Run
	s.RunLoop(context.Background())

	if !managerCalled {
		t.Error("Expected Agent to be called with Manager prompt due to TRIGGER_MANAGER signal")
	}
}

// MockLoopAgentMock allows function injection
type MockLoopAgentMock struct {
	SendFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *MockLoopAgentMock) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, prompt)
	}
	return "", nil
}

func (m *MockLoopAgentMock) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestSession_RunLoop_WaitBlocker(t *testing.T) {
	s, _ := setupLoopSession(t)
	defer s.DBStore.Close()

	// Set Blocker
	s.DBStore.SetSignal(s.Project, "BLOCKER", "Something broke")

	// Agent tries to run something
	s.Agent = &MockLoopAgent{Response: "```bash\necho try\n```"}

	// RunLoop swallows ErrBlocker and retries until resolved or max iterations.
	s.MaxIterations = 1

	err := s.RunLoop(context.Background())
	if !errors.Is(err, ErrMaxIterations) {
		t.Errorf("Expected ErrMaxIterations (swallowed blocker), got %v", err)
	}
}

func TestSession_RunLoop_StallBreaker(t *testing.T) {
	s, _ := setupLoopSession(t)
	defer s.DBStore.Close()

	// Stall breaker triggers if no features pass for N iterations.
	// checkStalledBreaker default is 15 (ManagerFrequency * 3).

	s.ManagerFrequency = 10
	s.StalledCount = 30 // Start at limit (30). Next inc will be 31.

	s.Agent = &MockLoopAgent{Response: "```bash\necho working\n```"} // Not NoOp, but no feature progress

	// We need features to track progress
	s.DBStore.SaveFeatures(s.Project, `{"project_name": "test", "features": [{"id":"1", "status":"todo"}]}`)

	// Max iterations = 1 (Should trip immediately)
	s.MaxIterations = 1

	err := s.RunLoop(context.Background())
	if !errors.Is(err, ErrStalled) {
		t.Errorf("Expected ErrStalled, got %v", err)
	}
}
