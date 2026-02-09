package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"recac/internal/notify"
	"recac/internal/security"
	"recac/internal/telemetry"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockScanner implements security.Scanner
type MockScanner struct {
	mock.Mock
}

func (m *MockScanner) Scan(content string) ([]security.Finding, error) {
	args := m.Called(content)
	return args.Get(0).([]security.Finding), args.Error(1)
}

func TestRunLoop_NoOp_Integrated(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	mockAgent := new(MockTestifyAgent)
	// Agent returns text but NO bash blocks -> NoOp
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("I am thinking...", nil)

	s := &Session{
		Workspace:        tmpDir,
		Agent:            mockAgent,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		MaxIterations:    5,
		ManagerFrequency: 10,
		SleepFunc:        func(d time.Duration) {}, // Fast sleep
	}

	err := s.RunLoop(context.Background())

	// Should fail with ErrNoOp after 3 strikes
	assert.ErrorIs(t, err, ErrNoOp)
}

func TestRunLoop_Stalled_Integrated(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	mockAgent := new(MockTestifyAgent)
	// Agent returns commands (valid op) but features don't change -> Stalled
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("Running command\n```bash\necho 1\n```", nil)
	mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).Return("Running command\n```bash\necho 1\n```", nil)

	mockDB := &MockRunLoopDBStore{
		GetFeaturesFunc: func(projectID string) (string, error) {
			// Always return same features, 0 passing
			return `{"features": [{"id": "1", "status": "todo", "passes": false}]}`, nil
		},
		SaveFeaturesFunc: func(projectID, features string) error { return nil },
		SaveObservationFunc: func(projectID, agentID, content string) error { return nil },
		GetSignalFunc: func(projectID, key string) (string, error) { return "", nil },
	}

	mockDocker := &MockLoopDocker{
		ExecFunc: func(ctx context.Context, containerID string, cmd []string) (string, error) {
			// If checking for blockers, return empty (no file)
			if len(cmd) > 0 && strings.Contains(cmd[len(cmd)-1], "blockers.txt") {
				return "", errors.New("file not found")
			}
			return "done", nil
		},
	}

	s := &Session{
		Workspace:        tmpDir,
		Agent:            mockAgent,
		Docker:           mockDocker,
		DBStore:          mockDB,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		MaxIterations:    20,
		ManagerFrequency: 10,
		StalledCount:     30, // Start high to trip immediately
		Iteration:        1,  // Not divisible by 10 to avoid Manager reset
		SleepFunc:        func(d time.Duration) {},
	}

	err := s.RunLoop(context.Background())

	assert.ErrorIs(t, err, ErrStalled)
}

func TestRunLoop_SecurityViolation(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	mockAgent := new(MockTestifyAgent)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("bad code", nil)

	mockScanner := new(MockScanner)
	mockScanner.On("Scan", "bad code").Return([]security.Finding{{Type: "Secret"}}, nil)

	sleepCalled := false
	s := &Session{
		Workspace:        tmpDir,
		Agent:            mockAgent,
		Scanner:          mockScanner,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		MaxIterations:    1,
		SleepFunc: func(d time.Duration) {
			sleepCalled = true
			assert.Equal(t, 5*time.Second, d)
		},
	}

	err := s.RunLoop(context.Background())

	assert.ErrorIs(t, err, ErrMaxIterations)
	assert.True(t, sleepCalled, "Should have slept for backoff")
}

func TestRunManagerAgent_Coverage(t *testing.T) {
	tmpDir := t.TempDir()

	mockAgent := new(MockTestifyAgent)
	// 1. Manager returns error
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("", errors.New("API error")).Once()

	s := &Session{
		Workspace:    tmpDir,
		ManagerAgent: mockAgent,
		Logger:       telemetry.NewLogger(true, "", false),
		Project:      "test-proj",
	}

	err := s.runManagerAgent(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manager review request failed")

	// 2. Manager returns success but no sign-off signal
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("Looks good but not signing off yet.", nil).Once()

	// Mock DB to return NO signal
	mockDB := &MockRunLoopDBStore{
		GetSignalFunc: func(projectID, key string) (string, error) {
			return "", nil
		},
		GetFeaturesFunc: func(projectID string) (string, error) {
			return `{"features": []}`, nil
		},
		DeleteSignalFunc: func(projectID, key string) error { return nil },
	}
	s.DBStore = mockDB

	err = s.runManagerAgent(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manager review did not result in sign-off")

	// 3. Manager signs off via signal
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("Signing off.", nil).Once()

	mockDB.GetSignalFunc = func(projectID, key string) (string, error) {
		if key == "PROJECT_SIGNED_OFF" {
			return "true", nil
		}
		return "", nil
	}

	err = s.runManagerAgent(context.Background())
	assert.NoError(t, err)
}

func TestRunQAAgent_Coverage(t *testing.T) {
	tmpDir := t.TempDir()

	mockAgent := new(MockTestifyAgent)
	// 1. QA Agent returns error
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("", errors.New("API error")).Once()

	s := &Session{
		Workspace: tmpDir,
		QAAgent:   mockAgent,
		Logger:    telemetry.NewLogger(true, "", false),
		Project:   "test-proj",
	}

	err := s.runQAAgent(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "QA Agent failed to respond")

	// 2. QA Agent returns success but explicit FALSE signal
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("Failed.", nil).Once()

	mockDB := &MockRunLoopDBStore{
		GetSignalFunc: func(projectID, key string) (string, error) {
			if key == "QA_PASSED" {
				return "false", nil
			}
			return "", nil
		},
		DeleteSignalFunc: func(projectID, key string) error { return nil },
	}
	s.DBStore = mockDB

	err = s.runQAAgent(context.Background())
	assert.Error(t, err)
	assert.Equal(t, "QA Agent explicitly signaled failure", err.Error())

	// 3. QA Agent returns success but NO signal (Implicit Fail)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("Done.", nil).Once()
	mockDB.GetSignalFunc = func(projectID, key string) (string, error) {
		return "", nil
	}

	err = s.runQAAgent(context.Background())
	assert.Error(t, err)
	assert.Equal(t, "QA Agent did not signal success (QA_PASSED!=true)", err.Error())
}

func TestSession_SetContainerID(t *testing.T) {
	s := &Session{}
	s.SetContainerID("123")
	assert.Equal(t, "123", s.GetContainerID())
}

func TestRunLoop_SkipQA(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

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
		GetFeaturesFunc: func(projectID string) (string, error) {
			return `{"features": []}`, nil
		},
	}

	s := &Session{
		Workspace:        tmpDir,
		DBStore:          mockDB,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		SkipQA:           true,
		MaxIterations:    2,
		SleepFunc:        func(d time.Duration) {},
	}

	err := s.RunLoop(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "true", signals["PROJECT_SIGNED_OFF"])
}

func TestRunLoop_ManagerFirst_InitialPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	mockAgent := new(MockTestifyAgent)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("Manager says proceed", nil)

	s := &Session{
		Workspace:        tmpDir,
		ManagerAgent:     mockAgent,
		Agent:            mockAgent, // RunIteration uses this one
		ManagerFirst:     true,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		MaxIterations:    1,
		SleepFunc:        func(d time.Duration) {},
	}

	err := s.RunLoop(context.Background())

	assert.ErrorIs(t, err, ErrMaxIterations)
	mockAgent.AssertCalled(t, "Send", mock.Anything, mock.Anything)
}
