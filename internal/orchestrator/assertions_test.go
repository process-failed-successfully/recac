package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestJobAssertions_Passing(t *testing.T) {
	poller := NewFilePoller("dummy.json")
	spawner := &assertionMockSpawner{coverage: 85.0, status: "success"}
	o := New(poller, spawner, time.Millisecond*100)
	spawner.o = o

	ctx := context.Background()
	logger := slog.Default()

	item := WorkItem{
		ID:          "job-pass-assertions",
		Summary:     "Test Job",
		Description: "A job to test assertions",
		Assertions: []string{
			"${metrics.coverage} >= 80.0",
			"'${outputs.status}' == 'success'",
		},
	}

	err := o.SubmitJob(ctx, item, logger)
	assert.NoError(t, err)

	o.wg.Wait()

	job, err := o.GetJob("job-pass-assertions")
	assert.NoError(t, err)
	assert.Equal(t, "Completed", job.Status)
}

func TestJobAssertions_Failing(t *testing.T) {
	poller := NewFilePoller("dummy.json")
	spawner := &assertionMockSpawner{coverage: 75.0, status: "success"}
	o := New(poller, spawner, time.Millisecond*100)
	spawner.o = o

	ctx := context.Background()
	logger := slog.Default()

	item := WorkItem{
		ID:          "job-fail-assertions",
		Summary:     "Test Job",
		Description: "A job to test assertions",
		Assertions: []string{
			"${metrics.coverage} >= 80.0",
			"'${outputs.status}' == 'success'",
		},
	}

	err := o.SubmitJob(ctx, item, logger)
	assert.NoError(t, err)

	o.wg.Wait()

	job, err := o.GetJob("job-fail-assertions")
	assert.NoError(t, err)
	assert.Equal(t, "Failed", job.Status)
	assert.Contains(t, job.Error, "assertion failed: ${metrics.coverage} >= 80.0")
}

type assertionMockSpawner struct {
	o        *Orchestrator
	coverage float64
	status   string
}

func (s *assertionMockSpawner) Spawn(ctx context.Context, item WorkItem) error {
	s.o.SetJobOutput(item.ID, map[string]string{"status": s.status}, nil)
	s.o.AddJobMetrics(item.ID, map[string]float64{"coverage": s.coverage}, nil)
	return nil
}
func (s *assertionMockSpawner) Cleanup(ctx context.Context, item WorkItem) error { return nil }
func (s *assertionMockSpawner) Cancel(ctx context.Context, jobID string) error   { return nil }
func (s *assertionMockSpawner) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) {
	return nil, nil
}
func (s *assertionMockSpawner) Ping(ctx context.Context) error { return nil }
