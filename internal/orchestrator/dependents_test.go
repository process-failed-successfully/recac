package orchestrator

import (

	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetJobDependents(t *testing.T) {
	o := New(nil, nil, 1*time.Minute)

	o.mu.Lock()
	// Job A
	o.completedJobs = append(o.completedJobs, JobInfo{
		ID:     "job-A",
		Status: "Completed",
		WorkItem: WorkItem{
			ID: "job-A",
		},
	})
	// Job B depends on A
	o.completedJobs = append(o.completedJobs, JobInfo{
		ID:     "job-B",
		Status: "Completed",
		WorkItem: WorkItem{
			ID:        "job-B",
			DependsOn: []string{"job-A"},
		},
	})
	// Job C depends on A and B
	o.activeJobs["job-C"] = JobInfo{
		ID:     "job-C",
		Status: "Running",
		WorkItem: WorkItem{
			ID:        "job-C",
			DependsOn: []string{"job-A", "job-B"},
		},
	}
	// Job D depends on C
	o.pendingJobs["job-D"] = JobInfo{
		ID:     "job-D",
		Status: "Pending",
		WorkItem: WorkItem{
			ID:        "job-D",
			DependsOn: []string{"job-C"},
		},
	}
	// Job E depends on nothing
	o.pendingJobs["job-E"] = JobInfo{
		ID:     "job-E",
		Status: "Pending",
		WorkItem: WorkItem{
			ID:        "job-E",
		},
	}
	o.mu.Unlock()

	// Dependents of A
	depsA, err := o.GetJobDependents("job-A")
	assert.NoError(t, err)
	assert.Len(t, depsA, 2)

	idsA := []string{depsA[0].ID, depsA[1].ID}
	assert.Contains(t, idsA, "job-B")
	assert.Contains(t, idsA, "job-C")

	// Dependents of B
	depsB, err := o.GetJobDependents("job-B")
	assert.NoError(t, err)
	assert.Len(t, depsB, 1)
	assert.Equal(t, "job-C", depsB[0].ID)

	// Dependents of C
	depsC, err := o.GetJobDependents("job-C")
	assert.NoError(t, err)
	assert.Len(t, depsC, 1)
	assert.Equal(t, "job-D", depsC[0].ID)

	// Dependents of D
	depsD, err := o.GetJobDependents("job-D")
	assert.NoError(t, err)
	assert.Len(t, depsD, 0)

	// Dependents of E
	depsE, err := o.GetJobDependents("job-E")
	assert.NoError(t, err)
	assert.Len(t, depsE, 0)

	// Dependents of non-existent job
	_, err = o.GetJobDependents("job-Z")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job-Z not found")
}
