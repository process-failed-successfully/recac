package orchestrator

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_GetCompletedJobs(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	// Initially empty
	assert.Empty(t, orch.GetCompletedJobs())

	// Add some jobs directly to history (simulating completion)
	job1 := JobInfo{ID: "JOB-1", Status: "Completed", EndTime: time.Now()}
	job2 := JobInfo{ID: "JOB-2", Status: "Failed", EndTime: time.Now()}

	orch.mu.Lock()
	orch.completedJobs = append(orch.completedJobs, job1, job2)
	orch.mu.Unlock()

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 2)
	assert.Equal(t, "JOB-1", completed[0].ID)
	assert.Equal(t, "JOB-2", completed[1].ID)

	// Verify it returns a copy
	completed[0].Status = "Modified"
	assert.Equal(t, "Completed", orch.GetCompletedJobs()[0].Status)
}

func TestOrchestrator_GetJob_Completed(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	job1 := JobInfo{ID: "JOB-1", Status: "Completed"}
	orch.mu.Lock()
	orch.completedJobs = append(orch.completedJobs, job1)
	orch.mu.Unlock()

	job, err := orch.GetJob("JOB-1")
	require.NoError(t, err)
	assert.Equal(t, "JOB-1", job.ID)
	assert.Equal(t, "Completed", job.Status)

	_, err = orch.GetJob("NON-EXISTENT")
	assert.Error(t, err)
}

func TestOrchestrator_CancelJob(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	ctx := context.Background()

	// Cancel existing job
	err := orch.CancelJob(ctx, "JOB-1")
	assert.NoError(t, err) // Mock spawner returns nil
}

func TestOrchestrator_GetLogs(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	ctx := context.Background()

	// Get logs
	logs, err := orch.GetLogs(ctx, "JOB-1")
	require.NoError(t, err)
	defer logs.Close()

	content, err := io.ReadAll(logs)
	require.NoError(t, err)
	assert.Equal(t, "mock logs", string(content))
}

func TestOrchestrator_JobHistoryLimit(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)
	orch.maxHistory = 2 // Set small limit

	// Add 3 jobs
	orch.addToHistory(JobInfo{ID: "JOB-1"}, nil)
	orch.addToHistory(JobInfo{ID: "JOB-2"}, nil)
	orch.addToHistory(JobInfo{ID: "JOB-3"}, nil)

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 2)
	assert.Equal(t, "JOB-2", completed[0].ID)
	assert.Equal(t, "JOB-3", completed[1].ID)
}

// TestSpawner implementation for custom error injection
type testSpawner struct {
	spawned   []WorkItem
	spawnErr  error
	cancelErr error
	logsErr   error
	pingErr   error
	mu        sync.Mutex
	blockCh   chan struct{}
}

func (m *testSpawner) Spawn(ctx context.Context, item WorkItem) error {
	if m.blockCh != nil {
		<-m.blockCh
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spawned = append(m.spawned, item)
	return m.spawnErr
}

func (m *testSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	return nil
}

func (m *testSpawner) Cancel(ctx context.Context, jobID string) error {
	return m.cancelErr
}

func (m *testSpawner) Ping(ctx context.Context) error {
	return m.pingErr
}

func (m *testSpawner) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) {
	if m.logsErr != nil {
		return nil, m.logsErr
	}
	return io.NopCloser(strings.NewReader("mock logs")), nil
}

func TestOrchestrator_CancelJob_WithCustomSpawner(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &testSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	ctx := context.Background()

	// Cancel existing job
	err := orch.CancelJob(ctx, "JOB-1")
	assert.NoError(t, err)

	// Simulate error in spawner
	spawner.cancelErr = errors.New("failed to cancel")
	err = orch.CancelJob(ctx, "JOB-2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to cancel")
}

func TestOrchestrator_GetLogs_WithCustomSpawner(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &testSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	ctx := context.Background()

	// Get logs
	logs, err := orch.GetLogs(ctx, "JOB-1")
	require.NoError(t, err)
	defer logs.Close()

	content, err := io.ReadAll(logs)
	require.NoError(t, err)
	assert.Equal(t, "mock logs", string(content))

	// Simulate error
	spawner.logsErr = errors.New("failed to get logs")
	_, err = orch.GetLogs(ctx, "JOB-2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get logs")
}

type mockDockerClient struct {
	cleanupCalls int
	mu           sync.Mutex
}

func (m *mockDockerClient) RunContainer(ctx context.Context, image string, workspace string, binds, env, cmd []string, user string) (string, error) {
	return "", nil
}

func (m *mockDockerClient) RunContainerWithLabels(ctx context.Context, image string, workspace string, binds, env, cmd []string, user string, labels map[string]string) (string, error) {
	return "", nil
}

func (m *mockDockerClient) StopContainer(ctx context.Context, containerID string) error {
	return nil
}

func (m *mockDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	return "", nil
}

func (m *mockDockerClient) ListContainers(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	m.mu.Lock()
	m.cleanupCalls++
	m.mu.Unlock()
	return []types.Container{}, nil
}

func (m *mockDockerClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	return nil
}

func (m *mockDockerClient) ContainerLogs(ctx context.Context, containerID string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockDockerClient) WaitContainer(ctx context.Context, containerID string) (int64, error) {
	return 0, nil
}

func (m *mockDockerClient) ImageExists(ctx context.Context, tag string) (bool, error) {
	return true, nil
}

func (m *mockDockerClient) PullImage(ctx context.Context, imageRef string) error {
	return nil
}

func TestJanitor_Start(t *testing.T) {
	// Mock Docker Client
	mockClient := &mockDockerClient{}

	// Use a small interval
	janitor := NewJanitor(silentLogger, mockClient, 10*time.Millisecond, 1*time.Hour, false, "")

	ctx, cancel := context.WithCancel(context.Background())

	// Start in goroutine
	done := make(chan struct{})
	go func() {
		janitor.Start(ctx)
		close(done)
	}()

	// Wait for a bit to let it run
	time.Sleep(50 * time.Millisecond)

	cancel()
	<-done

	mockClient.mu.Lock()
	calls := mockClient.cleanupCalls
	mockClient.mu.Unlock()
	assert.Greater(t, calls, 0)
}
