package orphan

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockRegistry struct {
	jobs []JobInfo
	err  error
}

func (m *mockRegistry) GetActiveJobs() ([]JobInfo, error) {
	return m.jobs, m.err
}

func (m *mockRegistry) GetJobStatus(jobID string) (JobStatus, error) {
	for _, job := range m.jobs {
		if job.ID == jobID {
			return job.Status, nil
		}
	}
	return "", errors.New("job not found")
}

func TestDetectOrphans(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		jobs []JobInfo
		wantOrphans int
	}{
		{
			name: "No orphans",
			jobs: []JobInfo{
				{ID: "1", Status: StatusRunning, AgentID: "agent-1", CreatedAt: now},
				{ID: "2", Status: StatusPending, AgentID: "agent-2", CreatedAt: now},
			},
			wantOrphans: 0,
		},
		{
			name: "Orphan without agent",
			jobs: []JobInfo{
				{ID: "1", Status: StatusRunning, AgentID: "", CreatedAt: now},
			},
			wantOrphans: 1,
		},
		{
			name: "Orphan stuck in pending",
			jobs: []JobInfo{
				{ID: "1", Status: StatusPending, AgentID: "agent-1", CreatedAt: now.Add(-2 * time.Hour)},
			},
			wantOrphans: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &mockRegistry{jobs: tt.jobs}
			detector := NewOrphanDetector(registry, time.Second, time.Hour)

			err := detector.detectOrphans()
			if err != nil {
				t.Fatalf("detectOrphans() error = %v", err)
			}
		})
	}
}

func TestScan(t *testing.T) {
	registry := &mockRegistry{
		jobs: []JobInfo{
			{ID: "1", Status: StatusRunning, AgentID: "", CreatedAt: time.Now()},
		},
	}
	detector := NewOrphanDetector(registry, 100*time.Millisecond, time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	detector.Scan(ctx)
}
