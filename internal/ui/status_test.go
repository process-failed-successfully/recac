package ui

import (
	"context"
	"errors"
	"recac/internal/k8s"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
)

// Mock Session Manager (Renamed to avoid conflict)
type statusMockSessionManager struct {
	listSessionsFunc func() ([]*runner.SessionState, error)
}

func (m *statusMockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	if m.listSessionsFunc != nil {
		return m.listSessionsFunc()
	}
	return nil, nil
}

// Implement other ISessionManager methods with dummies
func (m *statusMockSessionManager) SaveSession(s *runner.SessionState) error             { return nil }
func (m *statusMockSessionManager) LoadSession(name string) (*runner.SessionState, error) { return nil, nil }
func (m *statusMockSessionManager) StopSession(name string) error                         { return nil }
func (m *statusMockSessionManager) PauseSession(name string) error                        { return nil }
func (m *statusMockSessionManager) ResumeSession(name string) error                       { return nil }
func (m *statusMockSessionManager) GetSessionLogs(name string) (string, error)            { return "", nil }
func (m *statusMockSessionManager) GetSessionLogContent(name string, lines int) (string, error) {
	return "", nil
}
func (m *statusMockSessionManager) StartSession(name, goal string, command []string, workspace string) (*runner.SessionState, error) {
	return nil, nil
}
func (m *statusMockSessionManager) GetSessionPath(name string) string           { return "" }
func (m *statusMockSessionManager) IsProcessRunning(pid int) bool               { return false }
func (m *statusMockSessionManager) RemoveSession(name string, force bool) error { return nil }
func (m *statusMockSessionManager) RenameSession(oldName, newName string) error { return nil }
func (m *statusMockSessionManager) SessionsDir() string                         { return "" }
func (m *statusMockSessionManager) GetSessionGitDiffStat(name string) (string, error) {
	return "", nil
}
func (m *statusMockSessionManager) ArchiveSession(name string) error   { return nil }
func (m *statusMockSessionManager) UnarchiveSession(name string) error { return nil }
func (m *statusMockSessionManager) ListArchivedSessions() ([]*runner.SessionState, error) {
	return nil, nil
}

// Mock Docker Client
type statusMockDockerClient struct {
	serverVersionFunc func(ctx context.Context) (types.Version, error)
}

func (m *statusMockDockerClient) ServerVersion(ctx context.Context) (types.Version, error) {
	if m.serverVersionFunc != nil {
		return m.serverVersionFunc(ctx)
	}
	return types.Version{}, nil
}

func TestGetStatus(t *testing.T) {
	// Restore originals
	originalSM := NewSessionManagerFunc
	defer func() { NewSessionManagerFunc = originalSM }()

	originalDocker := NewDockerClientFunc
	defer func() { NewDockerClientFunc = originalDocker }()

	originalK8s := K8sNewClient
	defer func() { K8sNewClient = originalK8s }()

	// 1. Success Case
	NewSessionManagerFunc = func() (runner.ISessionManager, error) {
		return &statusMockSessionManager{
			listSessionsFunc: func() ([]*runner.SessionState, error) {
				return []*runner.SessionState{
					{Name: "session1", PID: 123, Status: "running", StartTime: time.Now().Add(-1 * time.Minute), LogFile: "log1"},
				}, nil
			},
		}, nil
	}

	NewDockerClientFunc = func(project string) (StatusDockerClient, error) {
		return &statusMockDockerClient{
			serverVersionFunc: func(ctx context.Context) (types.Version, error) {
				return types.Version{Version: "20.10.0", APIVersion: "1.41", Os: "linux", Arch: "amd64"}, nil
			},
		}, nil
	}

	K8sNewClient = func() (*k8s.Client, error) {
		return nil, errors.New("k8s not available")
	}

	// Test SetK8sClient
	SetK8sClient(K8sNewClient)

	output := GetStatus()
	if !strings.Contains(output, "session1") {
		t.Error("Expected session1 in status")
	}
	if !strings.Contains(output, "Docker Version: 20.10.0") {
		t.Error("Expected Docker Version")
	}
	if !strings.Contains(output, "Could not connect to Kubernetes") {
		t.Error("Expected K8s error message")
	}

	// 2. Error Cases
	NewSessionManagerFunc = func() (runner.ISessionManager, error) {
		return nil, errors.New("sm error")
	}
	NewDockerClientFunc = func(project string) (StatusDockerClient, error) {
		return nil, errors.New("docker error")
	}

	outputErr := GetStatus()
	if !strings.Contains(outputErr, "failed to initialize session manager") {
		t.Error("Expected session manager error")
	}
	if !strings.Contains(outputErr, "Docker client failed to initialize") {
		t.Error("Expected Docker error")
	}
}
