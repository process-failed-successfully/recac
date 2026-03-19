package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockDynamicSpawner struct {
	spawned   bool
	spawnFunc func(item WorkItem)
}

func (m *mockDynamicSpawner) Spawn(ctx context.Context, item WorkItem) error {
	m.spawned = true
	if m.spawnFunc != nil {
		m.spawnFunc(item)
	}
	return nil
}
func (m *mockDynamicSpawner) Cleanup(ctx context.Context, item WorkItem) error { return nil }
func (m *mockDynamicSpawner) Cancel(ctx context.Context, jobID string) error   { return nil }
func (m *mockDynamicSpawner) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) {
	return nil, nil
}
func (m *mockDynamicSpawner) Ping(ctx context.Context) error { return nil }

func TestOrchestrator_DynamicSpawnJobs(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockDynamicSpawner{}

	orch := New(poller, spawner, 1*time.Minute)

	// Intercept spawn to dynamically set output
	spawner.spawnFunc = func(item WorkItem) {
		if item.ID == "PARENT-JOB" {
			newJobs := []WorkItem{
				{ID: "CHILD-JOB-1", Summary: "Child 1", DependsOn: []string{"PARENT-JOB"}},
				{ID: "CHILD-JOB-2", Summary: "Child 2", DependsOn: []string{"PARENT-JOB"}},
			}
			newJobsJSON, _ := json.Marshal(newJobs)
			orch.SetJobOutput("PARENT-JOB", map[string]string{
				"RECAC_SPAWN_JOBS": string(newJobsJSON),
			}, nil)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go orch.Run(ctx, slog.Default())

	// Wait for start
	time.Sleep(10 * time.Millisecond)

	// Submit parent
	err := orch.SubmitJob(ctx, WorkItem{ID: "PARENT-JOB", Summary: "Parent"}, slog.Default())
	assert.NoError(t, err)

	// Give it time to spawn parent, complete, and dynamically spawn children
	time.Sleep(100 * time.Millisecond)

	// Check if children were spawned
	child1, err1 := orch.GetJob("CHILD-JOB-1")
	assert.NoError(t, err1)
	assert.Equal(t, "CHILD-JOB-1", child1.ID)

	child2, err2 := orch.GetJob("CHILD-JOB-2")
	assert.NoError(t, err2)
	assert.Equal(t, "CHILD-JOB-2", child2.ID)
}

func TestOrchestrator_DynamicSpawnJobs_InvalidJSON(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockDynamicSpawner{}

	orch := New(poller, spawner, 1*time.Minute)

	spawner.spawnFunc = func(item WorkItem) {
		if item.ID == "PARENT-JOB-ERR" {
			orch.SetJobOutput("PARENT-JOB-ERR", map[string]string{
				"RECAC_SPAWN_JOBS": "invalid json",
			}, nil)
		}
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	go orch.Run(ctx2, slog.Default())

	time.Sleep(10 * time.Millisecond)

	err2 := orch.SubmitJob(ctx2, WorkItem{ID: "PARENT-JOB-ERR", Summary: "Parent Err"}, slog.Default())
	assert.NoError(t, err2)

	time.Sleep(100 * time.Millisecond)

	job, err := orch.GetJob("PARENT-JOB-ERR")
	assert.NoError(t, err)
	assert.Equal(t, "Completed", job.Status)
	assert.Equal(t, "invalid json", job.Outputs["RECAC_SPAWN_JOBS"])
}
