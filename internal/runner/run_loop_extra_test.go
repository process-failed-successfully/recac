package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/db"
	"recac/internal/git"
	"recac/internal/notify"
	"recac/internal/telemetry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Reusing MockDockerClient from mock_docker_test.go
// Reusing MockGitClient from session_manager_test.go

// MockRunLoopDBStore implements db.Store with injection support
type MockRunLoopDBStore struct {
	GetSignalFunc       func(projectID, key string) (string, error)
	SetSignalFunc       func(projectID, key, value string) error
	DeleteSignalFunc    func(projectID, key string) error
	GetFeaturesFunc     func(projectID string) (string, error)
	SaveFeaturesFunc    func(projectID, features string) error
	SaveObservationFunc func(projectID, agentID, content string) error
	SaveSpecFunc        func(projectID, spec string) error
	GetSpecFunc         func(projectID string) (string, error)
	QueryHistoryFunc    func(projectID string, limit int) ([]db.Observation, error)
	AcquireLockFunc     func(projectID, path, agentID string, timeout time.Duration) (bool, error)
	ReleaseLockFunc     func(projectID, path, agentID string) error
	UpdateFeatureStatusFunc func(projectID, id, status string, passes bool) error
}

func (m *MockRunLoopDBStore) Close() error { return nil }
func (m *MockRunLoopDBStore) SaveObservation(projectID, agentID, content string) error {
	if m.SaveObservationFunc != nil {
		return m.SaveObservationFunc(projectID, agentID, content)
	}
	return nil
}
func (m *MockRunLoopDBStore) QueryHistory(projectID string, limit int) ([]db.Observation, error) {
	if m.QueryHistoryFunc != nil {
		return m.QueryHistoryFunc(projectID, limit)
	}
	return nil, nil
}
func (m *MockRunLoopDBStore) SetSignal(projectID, key, value string) error {
	if m.SetSignalFunc != nil {
		return m.SetSignalFunc(projectID, key, value)
	}
	return nil
}
func (m *MockRunLoopDBStore) GetSignal(projectID, key string) (string, error) {
	if m.GetSignalFunc != nil {
		return m.GetSignalFunc(projectID, key)
	}
	return "", nil
}
func (m *MockRunLoopDBStore) DeleteSignal(projectID, key string) error {
	if m.DeleteSignalFunc != nil {
		return m.DeleteSignalFunc(projectID, key)
	}
	return nil
}
func (m *MockRunLoopDBStore) SaveFeatures(projectID, features string) error {
	if m.SaveFeaturesFunc != nil {
		return m.SaveFeaturesFunc(projectID, features)
	}
	return nil
}
func (m *MockRunLoopDBStore) GetFeatures(projectID string) (string, error) {
	if m.GetFeaturesFunc != nil {
		return m.GetFeaturesFunc(projectID)
	}
	return "", nil
}
func (m *MockRunLoopDBStore) SaveSpec(projectID string, spec string) error {
	if m.SaveSpecFunc != nil {
		return m.SaveSpecFunc(projectID, spec)
	}
	return nil
}
func (m *MockRunLoopDBStore) GetSpec(projectID string) (string, error) {
	if m.GetSpecFunc != nil {
		return m.GetSpecFunc(projectID)
	}
	return "", nil
}
func (m *MockRunLoopDBStore) UpdateFeatureStatus(projectID, id string, status string, passes bool) error {
	if m.UpdateFeatureStatusFunc != nil {
		return m.UpdateFeatureStatusFunc(projectID, id, status, passes)
	}
	return nil
}
func (m *MockRunLoopDBStore) AcquireLock(projectID, path, agentID string, timeout time.Duration) (bool, error) {
	if m.AcquireLockFunc != nil {
		return m.AcquireLockFunc(projectID, path, agentID, timeout)
	}
	return true, nil
}
func (m *MockRunLoopDBStore) ReleaseLock(projectID, path, agentID string) error {
	if m.ReleaseLockFunc != nil {
		return m.ReleaseLockFunc(projectID, path, agentID)
	}
	return nil
}
func (m *MockRunLoopDBStore) ReleaseAllLocks(projectID, agentID string) error   { return nil }
func (m *MockRunLoopDBStore) GetActiveLocks(projectID string) ([]db.Lock, error) { return nil, nil }
func (m *MockRunLoopDBStore) Cleanup() error                                    { return nil }

// MockAgent implements agent.Agent with testify/mock for better control
type MockTestifyAgent struct {
	mock.Mock
}

func (m *MockTestifyAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockTestifyAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestRunLoop_Blocker(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	// Mock DB to return blocker
	mockDB := &MockRunLoopDBStore{
		GetSignalFunc: func(projectID, key string) (string, error) {
			if key == "BLOCKER" {
				return "Blocked by dependency", nil
			}
			return "", nil
		},
	}

	mockAgent := new(MockTestifyAgent)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("I will do something", nil)

	s := &Session{
		Workspace:     tmpDir,
		DBStore:       mockDB,
		Agent:         mockAgent,
		Notifier:      notify.NewManager(func(string, ...interface{}) {}),
		Logger:        telemetry.NewLogger(true, "", false),
		MaxIterations: 5,
	}

	// Execution
	err := s.RunLoop(context.Background())

	// Verification
	// RunLoop swallows ErrBlocker and retries until MaxIterations
	assert.ErrorIs(t, err, ErrMaxIterations)
}

func TestRunLoop_QAWorkflow(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	// Signals state
	signals := map[string]string{
		"COMPLETED": "true",
	}

	mockDB := &MockRunLoopDBStore{
		GetSignalFunc: func(projectID, key string) (string, error) {
			if val, ok := signals[key]; ok {
				return val, nil
			}
			return "", nil
		},
		SetSignalFunc: func(projectID, key, value string) error {
			signals[key] = value
			return nil
		},
		DeleteSignalFunc: func(projectID, key string) error {
			delete(signals, key)
			return nil
		},
		// Mock GetFeatures to avoid errors
		GetFeaturesFunc: func(projectID string) (string, error) {
			return `{"features": [{"id": "1", "description": "feat", "status": "done"}]}`, nil
		},
	}

	// Mock Agents
	mockQA := new(MockTestifyAgent)
	mockQA.On("Send", mock.Anything, mock.Anything).Return("QA Passed. No issues.", nil)

	mockManager := new(MockTestifyAgent)
	mockManager.On("Send", mock.Anything, mock.Anything).Return("Approved.", nil)

	s := &Session{
		Workspace:        tmpDir,
		DBStore:          mockDB,
		QAAgent:          mockQA,
		ManagerAgent:     mockManager,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		MaxIterations:    5,
		ManagerFrequency: 10,
	}

	// We need to simulate the loop checking QA passed.
	// 1. Loop sees COMPLETED -> runs runQAAgent.
	// 2. runQAAgent calls s.QAAgent.Send() (mocked).
	// 3. runQAAgent checks DB signal QA_PASSED.
	// Note: runQAAgent logic:
	//   Agent responds -> ProcessResponse -> Check DB "QA_PASSED".
	//   Since ProcessResponse executes commands (which might set DB signal via CLI),
	//   we need to either mock ProcessResponse behavior (hard via Session methods)
	//   OR Mock DB to return QA_PASSED="true" on subsequent calls.

	// Hack: We can't easily change DB state *during* the function call inside RunLoop unless we use a closure or channel in the mock.
	// But `runQAAgent` calls `s.QAAgent.Send`, then `s.ProcessResponse`.
	// `ProcessResponse` parses bash blocks. If mockQA returns a bash block `recac signal set QA_PASSED true`?
	// But `recac` binary might not exist or work in test environment.
	// Better: Mock DB `GetSignal` to return "true" for QA_PASSED *after* it has been queried once?
	// Or rely on the fallback: "QA Agent did not signal success (QA_PASSED!=true)".
	// Wait, runQAAgent returns error if signal not found.

	// Let's make `GetSignal` smart.
	qaCheckCount := 0
	mockDB.GetSignalFunc = func(projectID, key string) (string, error) {
		if key == "QA_PASSED" {
			qaCheckCount++
			if qaCheckCount > 1 { // Return true on second check (verification)
				return "true", nil
			}
		}
		if val, ok := signals[key]; ok {
			return val, nil
		}
		return "", nil
	}

	// Run
	// Note: This runs until MaxIterations (2).
	// Iter 1: sees COMPLETED, runs QA. QA passes (mocked DB hack). Sets QA_PASSED?
	//         If QA passes, it `continue`s loop.
	// Iter 2: sees QA_PASSED, runs Manager. Manager approves. Sets PROJECT_SIGNED_OFF.
	//         If Manager passes, it `continue`s loop.
	// Iter 3: Returns ErrMaxIterations (or nil if completed?)
	//         Actually RunLoop returns nil if PROJECT_SIGNED_OFF is set and cleaner runs.

	err := s.RunLoop(context.Background())

	// Verification
	// Should finish without error (nil) because PROJECT_SIGNED_OFF leads to return nil.
	assert.NoError(t, err)
	assert.Equal(t, "true", signals["PROJECT_SIGNED_OFF"])
	mockQA.AssertExpectations(t)
	mockManager.AssertExpectations(t)
}

func TestRunLoop_AutoMerge(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	// Initialize real git repo for direct exec calls in RunLoop
	exec.Command("git", "-C", tmpDir, "init").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", tmpDir, "commit", "--allow-empty", "-m", "init").Run()
	exec.Command("git", "-C", tmpDir, "checkout", "-b", "feature/foo").Run()

	// Mock DB
	mockDB := &MockRunLoopDBStore{
		GetSignalFunc: func(projectID, key string) (string, error) {
			if key == "PROJECT_SIGNED_OFF" {
				return "true", nil
			}
			return "", nil
		},
		GetFeaturesFunc: func(projectID string) (string, error) {
			return `{"features": []}`, nil
		},
	}

	// Mock Git
	mockGit := new(MockGitClient)
	// Safeguard checks
	mockGit.On("Fetch", mock.Anything, "origin", "main").Return(nil)
	mockGit.On("Stash", mock.Anything).Return(nil)
	mockGit.On("Merge", mock.Anything, "origin/main").Return(nil) // Merge upstream first
	mockGit.On("StashPop", mock.Anything).Return(nil)

	// Expect calls for AutoMerge
	// RepoExists and CurrentBranch are skipped or done via exec in this flow
	mockGit.On("Checkout", mock.Anything, "main").Return(nil)
	mockGit.On("Merge", mock.Anything, "feature/foo").Return(nil)
	mockGit.On("Push", mock.Anything, "main").Return(nil)
	mockGit.On("DeleteRemoteBranch", mock.Anything, "origin", "feature/foo").Return(nil)
	// Expect checkout back to feature branch
	mockGit.On("Checkout", mock.Anything, "feature/foo").Return(nil)

	mockGit.On("Commit", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Override git.NewClient
	originalNewClient := git.NewClient
	git.NewClient = func() git.IClient {
		return mockGit
	}
	defer func() { git.NewClient = originalNewClient }()

	s := &Session{
		Workspace:     tmpDir,
		DBStore:       mockDB,
		Notifier:      notify.NewManager(func(string, ...interface{}) {}),
		Logger:        telemetry.NewLogger(true, "", false),
		BaseBranch:    "main",
		AutoMerge:     true,
		RepoURL:       "http://github.com/org/repo",
		Project:       "test-proj",
		MaxIterations: 1,
	}

	// Execution
	err := s.RunLoop(context.Background())

	// Verification
	assert.NoError(t, err)
	mockGit.AssertExpectations(t)
}

func TestRunLoop_GitSafeguard_MergeConflict(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	signals := map[string]string{
		"PROJECT_SIGNED_OFF": "true",
	}

	mockDB := &MockRunLoopDBStore{
		GetSignalFunc: func(projectID, key string) (string, error) {
			if val, ok := signals[key]; ok {
				return val, nil
			}
			return "", nil
		},
		DeleteSignalFunc: func(projectID, key string) error {
			delete(signals, key)
			return nil
		},
		GetFeaturesFunc: func(projectID string) (string, error) {
			return `{"features": []}`, nil
		},
		SaveFeaturesFunc: func(projectID, features string) error {
			return nil
		},
	}

	// Mock Git
	mockGit := new(MockGitClient)
	// Safeguard check
	mockGit.On("Fetch", mock.Anything, "origin", "main").Return(nil)
	mockGit.On("Stash", mock.Anything).Return(nil)
	// Fail merge
	mockGit.On("Merge", mock.Anything, "origin/main").Return(errors.New("conflict"))
	// Abort
	mockGit.On("AbortMerge", mock.Anything).Return(nil)
	// Recovery attempts (simplified expectation: it retries or eventually fails)
	// We expect multiple calls maybe?
	mockGit.On("Recover", mock.Anything).Return(nil).Maybe()
	mockGit.On("Clean", mock.Anything).Return(nil).Maybe()

	// If brutal recovery kicks in (after 3 retries):
	mockGit.On("DeleteRemoteBranch", mock.Anything, "origin", mock.Anything).Return(nil).Maybe()
	mockGit.On("ResetHard", mock.Anything, "origin", "main").Return(nil).Maybe()

	// Override git.NewClient
	originalNewClient := git.NewClient
	git.NewClient = func() git.IClient {
		return mockGit
	}
	defer func() { git.NewClient = originalNewClient }()

	s := &Session{
		Workspace:     tmpDir,
		DBStore:       mockDB,
		Notifier:      notify.NewManager(func(string, ...interface{}) {}),
		Logger:        telemetry.NewLogger(true, "", false),
		BaseBranch:    "main",
		AutoMerge:     true,
		Project:       "test-proj",
		MaxIterations: 1, // Only one iteration needed to trigger safeguard and fail
	}

	// Execution
	// RunLoop should clear PROJECT_SIGNED_OFF and continue.
	// Since MaxIterations=1, it will finish loop with ErrMaxIterations or nil if it returns early?
	// It hits "continue" after clearing signal. Then hits max iterations check.
	err := s.RunLoop(context.Background())

	// Verification
	// Should return ErrMaxIterations because it loops back.
	assert.ErrorIs(t, err, ErrMaxIterations)

	// Signal should be cleared
	assert.Equal(t, "", signals["PROJECT_SIGNED_OFF"])

	// Verify git calls
	mockGit.AssertCalled(t, "Merge", tmpDir, "origin/main")
}

func TestSession_LoadFeatures_Priority(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()

	// 1. Env Injection (Highest Priority)
	envFeatures := `{"features": [{"id": "env", "description": "from env"}]}`
	t.Setenv("RECAC_INJECTED_FEATURES", envFeatures)

	// Mock DB
	mockDB := &MockRunLoopDBStore{
		GetFeaturesFunc: func(projectID string) (string, error) {
			return `{"features": [{"id": "db", "description": "from db"}]}`, nil
		},
		SaveFeaturesFunc: func(projectID, features string) error {
			return nil
		},
	}

	s := &Session{
		Workspace: tmpDir,
		DBStore:   mockDB,
		Project:   "test-proj",
		Logger:    telemetry.NewLogger(true, "", false),
	}

	// Execution: loadFeatures is private, but we can trigger it via SelectPrompt or RunLoop?
	// Or just test it if we export it or use reflection?
	// Since we are in 'package runner', we can call private methods!
	features := s.loadFeatures()

	// Verification
	// Should have merged Env + DB.
	// Env features should be present.
	foundEnv := false
	for _, f := range features {
		if f.ID == "env" {
			foundEnv = true
		}
	}
	assert.True(t, foundEnv, "Env features should be loaded")

	// Verify it synced to file
	content, err := os.ReadFile(filepath.Join(tmpDir, "feature_list.json"))
	assert.NoError(t, err)
	assert.Contains(t, string(content), "from env")
}

func TestRunLoop_SubTask(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	// Mock DB to return feature list where task becomes done
	callCount := 0
	mockDB := &MockRunLoopDBStore{
		GetFeaturesFunc: func(projectID string) (string, error) {
			callCount++
			// 1. Startup check
			// 2. Loop 1 check (Needs to be Todo to run iteration)
			// 3. Loop 2 check (Can be Done to exit)
			if callCount <= 2 {
				// Initial state: Not done
				return `{"features": [{"id": "task-1", "status": "todo", "passes": false}]}`, nil
			}
			// Subsequent state: Done (simulating agent update)
			return `{"features": [{"id": "task-1", "status": "done", "passes": true}]}`, nil
		},
		SaveFeaturesFunc: func(projectID, features string) error {
			return nil
		},
		// Signal checks
		GetSignalFunc: func(projectID, key string) (string, error) {
			return "", nil
		},
	}

	// Mock Agent - does some work then task is marked done (via DB mock)
	mockAgent := new(MockTestifyAgent)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("Working on subtask...", nil)

	s := &Session{
		Workspace:      tmpDir,
		DBStore:        mockDB,
		Agent:          mockAgent,
		Project:        "test-proj",
		SelectedTaskID: "task-1", // Focus on this task
		Logger:         telemetry.NewLogger(true, "", false),
		MaxIterations:  5,
		ManagerFrequency: 5,
	}

	// Execution
	err := s.RunLoop(context.Background())

	// Verification
	// Should exit with nil because task completed
	assert.NoError(t, err)

	// Verify agent was called (at least once)
	mockAgent.AssertNumberOfCalls(t, "Send", 1)
}

func TestSession_RunInitScript_Local(t *testing.T) {
	tmpDir := t.TempDir()

	// Create init.sh that creates a marker file
	initScript := filepath.Join(tmpDir, "init.sh")
	markerFile := filepath.Join(tmpDir, "init_ran")
	scriptContent := "#!/bin/sh\ntouch " + markerFile
	os.WriteFile(initScript, []byte(scriptContent), 0755)

	s := &Session{
		Workspace:     tmpDir,
		UseLocalAgent: true,
		Logger:        telemetry.NewLogger(true, "", false),
	}

	// Execution
	// runInitScript runs async in background. We need to wait for it.
	// But runInitScript method returns immediately.
	// It uses a goroutine.
	// We can't wait for goroutine easily without synchronization primitives not exposed.
	// However, it runs async but we can wait for file creation with timeout.

	s.runInitScript(context.Background())

	// Wait for marker file
	assert.Eventually(t, func() bool {
		_, err := os.Stat(markerFile)
		return err == nil
	}, 2*time.Second, 100*time.Millisecond, "init.sh should have run and created marker file")
}
