package orchestrator

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"log/slog"
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

// Extra mock persistence for GetJobDependents

type extraMockPersistence struct {
	jobs []JobInfo
}

func newExtraMockPersistence() *extraMockPersistence {
	return &extraMockPersistence{}
}

func (m *extraMockPersistence) Init() error { return nil }
func (m *extraMockPersistence) SaveJob(job JobInfo) error {
	m.jobs = append(m.jobs, job)
	return nil
}
func (m *extraMockPersistence) GetJobs(limit int) ([]JobInfo, error) {
	return m.jobs, nil
}
func (m *extraMockPersistence) GetJob(id string) (*JobInfo, error) {
	for _, j := range m.jobs {
		if j.ID == id {
			return &j, nil
		}
	}
	return nil, errors.New("not found")
}
func (m *extraMockPersistence) ClearHistory() (int, error) { return 0, nil }
func (m *extraMockPersistence) Close() error { return nil }
func (m *extraMockPersistence) PurgeJob(id string) error { return nil }

type extraDummySpawner struct{}
func (d *extraDummySpawner) Spawn(ctx context.Context, item WorkItem) error { return nil }
func (d *extraDummySpawner) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) { return nil, nil }
func (d *extraDummySpawner) Cancel(ctx context.Context, jobID string) error { return nil }
func (d *extraDummySpawner) Ping(ctx context.Context) error { return nil }
func (d *extraDummySpawner) Cleanup(ctx context.Context, item WorkItem) error { return nil }

func TestGetJobDependents_Extra(t *testing.T) {
	o := New(nil, &extraDummySpawner{}, 1*time.Minute)
	o.Persistence = newExtraMockPersistence()

	// Add jobs
	job1 := WorkItem{ID: "job1"}
	job2 := WorkItem{ID: "job2", DependsOn: []string{"job1"}}
	job3 := WorkItem{ID: "job3", DependsOn: []string{"job2"}}
	job4 := WorkItem{ID: "job4", DependsOn: []string{"job1"}}

	err := o.SubmitJob(context.Background(), job1, slog.Default())
	require.NoError(t, err)
	err = o.SubmitJob(context.Background(), job2, slog.Default())
	require.NoError(t, err)

	o.mu.Lock()
	// manually force them to different states to test the internal slices
	job2Info := o.pendingJobs["job2"]
	delete(o.pendingJobs, "job2")
	o.activeJobs["job2"] = job2Info

	o.completedJobs = append(o.completedJobs, JobInfo{
		ID:       "job3",
		WorkItem: job3,
	})
	o.mu.Unlock()

	// Add job4 to mock persistence to test the fallback logic
	err = o.Persistence.(*extraMockPersistence).SaveJob(JobInfo{ID: "job4", WorkItem: job4})
	require.NoError(t, err)

	deps, err := o.GetJobDependents("job1")
	require.NoError(t, err)

	assert.Len(t, deps, 2)

	foundJob2 := false
	foundJob4 := false
	for _, dep := range deps {
		if dep.ID == "job2" {
			foundJob2 = true
		} else if dep.ID == "job4" {
			foundJob4 = true
		}
	}
	assert.True(t, foundJob2, "job2 not found in dependents")
	assert.True(t, foundJob4, "job4 not found in dependents")

	// Missing job test
	_, err = o.GetJobDependents("job-not-exist")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUpdateJobDependencies(t *testing.T) {
	o := New(nil, &extraDummySpawner{}, 1*time.Minute)

	job1 := WorkItem{ID: "job1", Hold: true}
	err := o.SubmitJob(context.Background(), job1, slog.Default())
	require.NoError(t, err)

	logger := slog.Default()

	// Success case
	err = o.UpdateJobDependencies(context.Background(), "job1", []string{"dep1"}, logger)
	require.NoError(t, err)

	o.mu.Lock()
	assert.Equal(t, []string{"dep1"}, o.pendingJobs["job1"].WorkItem.DependsOn)
	o.mu.Unlock()

	// Nonexistent job case
	err = o.UpdateJobDependencies(context.Background(), "nonexistent", []string{"dep1"}, logger)
	assert.ErrorContains(t, err, "not found in pending queue")

	// Active job case
	o.mu.Lock()
	job1Info := o.pendingJobs["job1"]
	delete(o.pendingJobs, "job1")
	o.activeJobs["job1"] = job1Info
	o.mu.Unlock()

	err = o.UpdateJobDependencies(context.Background(), "job1", []string{"dep1"}, logger)
	assert.ErrorContains(t, err, "already active")

	// Completed job case
	o.mu.Lock()
	delete(o.activeJobs, "job1")
	o.completedJobs = append(o.completedJobs, job1Info)
	o.mu.Unlock()

	err = o.UpdateJobDependencies(context.Background(), "job1", []string{"dep1"}, logger)
	assert.ErrorContains(t, err, "already completed")
}

func TestHoldJobsByMatch_Extra(t *testing.T) {
	o := New(nil, &extraDummySpawner{}, 1*time.Minute)
	o.Persistence = newExtraMockPersistence()

	job1 := WorkItem{ID: "job1", Summary: "test summary 1", Hold: true}
	job2 := WorkItem{ID: "job2", Summary: "other info", Hold: true}
	job3 := WorkItem{ID: "job3", Hold: true}

	o.SubmitJob(context.Background(), job1, slog.Default())
	o.SubmitJob(context.Background(), job2, slog.Default())
	o.SubmitJob(context.Background(), job3, slog.Default())

	o.mu.Lock()
	info := o.pendingJobs["job3"]
	info.Error = "test error string"
	info.WorkItem.Hold = false
	o.pendingJobs["job3"] = info

	info1 := o.pendingJobs["job1"]
	info1.WorkItem.Hold = false
	o.pendingJobs["job1"] = info1
	o.mu.Unlock()

	logger := slog.Default()
	count, err := o.HoldJobsByMatch(context.Background(), "test", logger)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	o.mu.Lock()
	assert.True(t, o.pendingJobs["job1"].WorkItem.Hold)
	assert.True(t, o.pendingJobs["job2"].WorkItem.Hold) // was already hold
	assert.True(t, o.pendingJobs["job3"].WorkItem.Hold)
	o.mu.Unlock()

	_, err = o.HoldJobsByMatch(context.Background(), "(invalid", logger)
	assert.Error(t, err)
}

func TestUpdateJobPriority_Extra(t *testing.T) {
	o := New(nil, &extraDummySpawner{}, 1*time.Minute)
	o.Persistence = newExtraMockPersistence()

	job1 := WorkItem{ID: "job1", Priority: 1, Hold: true}
	o.SubmitJob(context.Background(), job1, slog.Default())

	logger := slog.Default()
	err := o.UpdateJobPriority(context.Background(), "job1", 5, logger)
	require.NoError(t, err)

	o.mu.Lock()
	assert.Equal(t, 5, o.pendingJobs["job1"].WorkItem.Priority)
	o.mu.Unlock()

	err = o.UpdateJobPriority(context.Background(), "nonexistent", 5, logger)
	assert.ErrorContains(t, err, "not found in pending queue")

	o.mu.Lock()
	job1Info := o.pendingJobs["job1"]
	delete(o.pendingJobs, "job1")
	o.activeJobs["job1"] = job1Info
	o.mu.Unlock()

	err = o.UpdateJobPriority(context.Background(), "job1", 5, logger)
	assert.ErrorContains(t, err, "already active")

	o.mu.Lock()
	delete(o.activeJobs, "job1")
	o.completedJobs = append(o.completedJobs, job1Info)
	o.mu.Unlock()

	err = o.UpdateJobPriority(context.Background(), "job1", 5, logger)
	assert.ErrorContains(t, err, "already completed")
}

func TestUpdateJobsPriorityByTag_Extra(t *testing.T) {
	o := New(nil, &extraDummySpawner{}, 1*time.Minute)

	job1 := WorkItem{ID: "job1", Tags: []string{"tag1"}, Hold: true}
	job2 := WorkItem{ID: "job2", Tags: []string{"tag2"}, Hold: true}

	o.SubmitJob(context.Background(), job1, slog.Default())
	o.SubmitJob(context.Background(), job2, slog.Default())

	logger := slog.Default()
	count, err := o.UpdateJobsPriorityByTag(context.Background(), "tag1", 10, logger)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	o.mu.Lock()
	assert.Equal(t, 10, o.pendingJobs["job1"].WorkItem.Priority)
	assert.Equal(t, 0, o.pendingJobs["job2"].WorkItem.Priority)
	o.mu.Unlock()
}
