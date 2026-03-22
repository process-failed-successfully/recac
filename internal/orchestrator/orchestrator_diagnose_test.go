package orchestrator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOrchestrator_Diagnose_NoIssues(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	report, err := orch.Diagnose(nil)
	assert.NoError(t, err)
	assert.Empty(t, report.UnresolvableJobs)
	assert.Empty(t, report.DeadlockedJobs)
}

func TestOrchestrator_Diagnose_UnresolvableMissing(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	// Use internal state manipulation instead of SubmitJob to avoid dependency checks triggering failure immediately
	orch.pendingJobs["job-missing-dep"] = JobInfo{
		ID:     "job-missing-dep",
		Status: "Pending",
		WorkItem: WorkItem{
			ID:        "job-missing-dep",
			DependsOn: []string{"ghost-job"},
		},
	}

	report, err := orch.Diagnose(nil)
	assert.NoError(t, err)
	assert.Empty(t, report.DeadlockedJobs)

	assert.Len(t, report.UnresolvableJobs, 1)
	assert.Equal(t, "job-missing-dep", report.UnresolvableJobs[0].JobID)
	assert.Contains(t, report.UnresolvableJobs[0].MissingDeps, "ghost-job")
	assert.Empty(t, report.UnresolvableJobs[0].FailedDeps)
	assert.Empty(t, report.UnresolvableJobs[0].CanceledDeps)
}

func TestOrchestrator_Diagnose_UnresolvableFailed(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	// Add a failed job to history
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:     "failed-job",
		Status: "Failed",
	})

	// Inject directly into pendingJobs
	orch.pendingJobs["job-failed-dep"] = JobInfo{
		ID:     "job-failed-dep",
		Status: "Pending",
		WorkItem: WorkItem{
			ID:        "job-failed-dep",
			DependsOn: []string{"failed-job"},
		},
	}

	report, err := orch.Diagnose(nil)
	assert.NoError(t, err)
	assert.Empty(t, report.DeadlockedJobs)

	assert.Len(t, report.UnresolvableJobs, 1)
	assert.Equal(t, "job-failed-dep", report.UnresolvableJobs[0].JobID)
	assert.Contains(t, report.UnresolvableJobs[0].FailedDeps, "failed-job")
	assert.Empty(t, report.UnresolvableJobs[0].MissingDeps)
	assert.Empty(t, report.UnresolvableJobs[0].CanceledDeps)
}

func TestOrchestrator_Diagnose_Deadlock(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	// Inject into pending jobs directly
	orch.pendingJobs["job-A"] = JobInfo{
		ID:     "job-A",
		Status: "Pending",
		WorkItem: WorkItem{
			ID:        "job-A",
			DependsOn: []string{"job-B"},
		},
	}
	orch.pendingJobs["job-B"] = JobInfo{
		ID:     "job-B",
		Status: "Pending",
		WorkItem: WorkItem{
			ID:        "job-B",
			DependsOn: []string{"job-A"},
		},
	}

	report, err := orch.Diagnose(nil)
	assert.NoError(t, err)

	// Both depend on each other, so neither is missing/failed/canceled
	assert.Empty(t, report.UnresolvableJobs)

	// They form a single unique cycle, but the cycle can start at A or B.
	// Our deterministic sort and deduplication should yield 1 unique cycle signature.
	assert.Len(t, report.DeadlockedJobs, 1)

	// Verify cycle contains both A and B
	cycle := report.DeadlockedJobs[0].Cycle
	assert.Len(t, cycle, 3) // e.g. A -> B -> A
	assert.Equal(t, cycle[0], cycle[2])

	if cycle[0] == "job-A" {
		assert.Equal(t, "job-B", cycle[1])
	} else {
		assert.Equal(t, "job-A", cycle[1])
	}
}

func TestOrchestrator_Diagnose_ComplexDeadlock(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	// Create a complex A -> B -> C -> A cycle + missing dep
	orch.pendingJobs["job-A"] = JobInfo{
		ID:     "job-A",
		Status: "Pending",
		WorkItem: WorkItem{ID: "job-A", DependsOn: []string{"job-B"}},
	}
	orch.pendingJobs["job-B"] = JobInfo{
		ID:     "job-B",
		Status: "Pending",
		WorkItem: WorkItem{ID: "job-B", DependsOn: []string{"job-C"}},
	}
	orch.pendingJobs["job-C"] = JobInfo{
		ID:     "job-C",
		Status: "Pending",
		WorkItem: WorkItem{ID: "job-C", DependsOn: []string{"job-A", "job-missing"}},
	}

	report, err := orch.Diagnose(nil)
	assert.NoError(t, err)

	// C is missing job-missing
	assert.Len(t, report.UnresolvableJobs, 1)
	assert.Equal(t, "job-C", report.UnresolvableJobs[0].JobID)
	assert.Contains(t, report.UnresolvableJobs[0].MissingDeps, "job-missing")

	assert.Len(t, report.DeadlockedJobs, 1)
	cycle := report.DeadlockedJobs[0].Cycle
	assert.Len(t, cycle, 4) // e.g. A -> B -> C -> A
	assert.Equal(t, cycle[0], cycle[3])
}
