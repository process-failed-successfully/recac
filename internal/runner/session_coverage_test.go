package runner

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/db"
	"recac/internal/docker"

	"github.com/docker/docker/api/types/image"
	"github.com/stretchr/testify/assert"
)

// We don't need ExtendedMockDocker anymore, we use docker.NewMockClient which gives us access to MockAPI.

func TestSession_EnsureImage_Pull(t *testing.T) {
	d, mockAPI := docker.NewMockClient()
	tmpDir := t.TempDir()

	session := NewSession(d, &MockAgent{}, tmpDir, "ghcr.io/process-failed-successfully/recac-agent:latest", "test-project", "gemini", "gemini-pro", 1)

	// Case 1: Image exists
	mockAPI.ImageListFunc = func(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
		return []image.Summary{{RepoTags: []string{"ghcr.io/process-failed-successfully/recac-agent:latest"}}}, nil
	}
	err := session.ensureImage(context.Background())
	assert.NoError(t, err)

	// Case 2: Image does not exist, pull success
	mockAPI.ImageListFunc = func(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
		return []image.Summary{}, nil
	}

	mockAPI.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
		return os.Open(os.DevNull) // Dummy reader
	}

	err = session.ensureImage(context.Background())
	assert.NoError(t, err)
}

func TestSession_EnsureImage_LegacyBuild(t *testing.T) {
	d, mockAPI := docker.NewMockClient()
	tmpDir := t.TempDir()

	session := NewSession(d, &MockAgent{}, tmpDir, "recac-agent:latest", "test-project", "gemini", "gemini-pro", 1)

	// Case: Image missing, build required
	mockAPI.ImageListFunc = func(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
		return []image.Summary{}, nil
	}
	// Build legacy image
	err := session.ensureImage(context.Background())
	assert.NoError(t, err)
}

// MockAgent that can return error
type ErrorMockAgent struct {
	MockAgent
	SendError error
}

func (m *ErrorMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendError != nil {
		return "", m.SendError
	}
	return m.MockAgent.Send(ctx, prompt)
}

// SideEffectMockAgent can perform an action when Send is called
type SideEffectMockAgent struct {
	MockAgent
	SideEffect func()
}

func (m *SideEffectMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.SideEffect != nil {
		m.SideEffect()
	}
	return m.MockAgent.Send(ctx, prompt)
}

func TestSession_RunQAAgent_Error(t *testing.T) {
	tmpDir := t.TempDir()
	mockAgent := &ErrorMockAgent{
		SendError: errors.New("agent failed"),
	}
	session := NewSession(nil, mockAgent, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.QAAgent = mockAgent

	err := session.runQAAgent(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent failed")
}

func TestSession_RunQAAgent_SignalSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	// Init DB
	dbStore, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: filepath.Join(tmpDir, ".recac.db")})

	mockAgent := &SideEffectMockAgent{}
	mockAgent.SideEffect = func() {
		// Set signal when agent "runs"
		dbStore.SetSignal("test-project", "QA_PASSED", "true")
	}

	session := NewSession(nil, mockAgent, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.QAAgent = mockAgent
	session.DBStore = dbStore
	session.Project = "test-project"

	// Agent returns junk, but signal set by side effect
	mockAgent.Response = "I passed the QA"

	err := session.runQAAgent(context.Background())
	assert.NoError(t, err)
}

func TestSession_RunManagerAgent_SignalSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	mockAgent := &MockAgent{}
	session := NewSession(nil, mockAgent, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.ManagerAgent = mockAgent

	// Init DB
	dbStore, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: filepath.Join(tmpDir, ".recac.db")})
	session.DBStore = dbStore
	session.Project = "test-project"

	// Set signal
	dbStore.SetSignal("test-project", "PROJECT_SIGNED_OFF", "true")

	// Create feature list (required for RunManagerAgent to generate report)
	os.WriteFile(filepath.Join(tmpDir, "feature_list.json"), []byte(`{"features":[]}`), 0644)

	err := session.runManagerAgent(context.Background())
	assert.NoError(t, err)
}

func TestSession_RunManagerAgent_Error(t *testing.T) {
	tmpDir := t.TempDir()
	mockAgent := &ErrorMockAgent{
		SendError: errors.New("manager failed"),
	}
	session := NewSession(nil, mockAgent, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.ManagerAgent = mockAgent

	os.WriteFile(filepath.Join(tmpDir, "feature_list.json"), []byte(`{"features":[]}`), 0644)

	err := session.runManagerAgent(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manager review request failed")
}

func TestSessionManager_Coverage(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := NewSessionManagerWithDir(tmpDir)
	assert.NoError(t, err)

	// Start a session (simulated)
	// We manually create state files
	stateDir := filepath.Join(tmpDir, "active", "test-session")
	os.MkdirAll(stateDir, 0755)

	// 1. IsProcessRunning (Mock PID)
	// It checks process. We can't easily mock process existence without helper.
	// But we can check StopSession behavior.

	// 2. StopSession
	// Should fail for non-existent session
	err = sm.StopSession("non-existent")
	assert.Error(t, err)

	// 3. RemoveSession
	err = sm.RemoveSession("test-session", true)
	// Actually RemoveSession expects a file named test-session.json in sessionsDir.
	// My setup in previous attempt: os.MkdirAll(filepath.Join(tmpDir, "active", "test-session"), 0755)
	// But SessionManager uses flat dir structure in sessionsDir (which is tmpDir).
	// So I should create tmpDir/test-session.json

	sessionFile := filepath.Join(tmpDir, "test-session.json")
	os.WriteFile(sessionFile, []byte(`{}`), 0644)

	err = sm.RemoveSession("test-session", true)
	assert.NoError(t, err)
	assert.NoFileExists(t, sessionFile)

	// 4. ListSessions
	// Create another one
	sessionFile2 := filepath.Join(tmpDir, "session-2.json")
	os.WriteFile(sessionFile2, []byte(`{"name":"session-2", "pid":999999, "status":"running"}`), 0644)

	sessions, err := sm.ListSessions()
	assert.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "session-2", sessions[0].Name)
	// Status might change to completed if PID not found, or dead?
	// ListSessions logic: if running and !IsProcessRunning -> completed.
	// Let's check.
	assert.Equal(t, "completed", sessions[0].Status)
}

func TestSession_FixPermissions_Local(t *testing.T) {
	session := &Session{
		ContainerID: "local",
		Docker: nil,
	}
	err := session.fixPermissions(context.Background())
	assert.NoError(t, err)
}

func TestSession_BootstrapGit_Local(t *testing.T) {
	session := &Session{
		UseLocalAgent: true,
		ContainerID: "local",
	}
	err := session.bootstrapGit(context.Background())
	assert.NoError(t, err)
}

func TestSession_RunInitScript_Local(t *testing.T) {
	tmpDir := t.TempDir()
	initPath := filepath.Join(tmpDir, "init.sh")
	os.WriteFile(initPath, []byte("#!/bin/sh\necho 'hello' > init_output.txt"), 0755)

	session := &Session{
		UseLocalAgent: true,
		Workspace: tmpDir,
	}

	session.runInitScript(context.Background())

	// Give it time to run
	time.Sleep(100 * time.Millisecond)

	assert.FileExists(t, filepath.Join(tmpDir, "init_output.txt"))
}
