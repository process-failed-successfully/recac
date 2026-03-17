package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_RenameJob(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		oldID     string
		newID     string
		setup     func(*Orchestrator)
		assertion func(*testing.T, *Orchestrator, error)
	}{
		{
			name:  "Same ID",
			oldID: "job-1",
			newID: "job-1",
			setup: func(o *Orchestrator) {},
			assertion: func(t *testing.T, o *Orchestrator, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:  "Job Not Found",
			oldID: "job-1",
			newID: "job-2",
			setup: func(o *Orchestrator) {},
			assertion: func(t *testing.T, o *Orchestrator, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "not found")
			},
		},
		{
			name:  "New ID Already Exists Pending",
			oldID: "job-1",
			newID: "job-2",
			setup: func(o *Orchestrator) {
				o.pendingJobs["job-1"] = JobInfo{ID: "job-1", WorkItem: WorkItem{ID: "job-1"}}
				o.pendingJobs["job-2"] = JobInfo{ID: "job-2", WorkItem: WorkItem{ID: "job-2"}}
			},
			assertion: func(t *testing.T, o *Orchestrator, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "already exists in pending queue")
			},
		},
		{
			name:  "New ID Already Exists Active",
			oldID: "job-1",
			newID: "job-2",
			setup: func(o *Orchestrator) {
				o.pendingJobs["job-1"] = JobInfo{ID: "job-1", WorkItem: WorkItem{ID: "job-1"}}
				o.activeJobs["job-2"] = JobInfo{ID: "job-2", WorkItem: WorkItem{ID: "job-2"}}
			},
			assertion: func(t *testing.T, o *Orchestrator, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "already active")
			},
		},
		{
			name:  "New ID Already Exists Completed",
			oldID: "job-1",
			newID: "job-2",
			setup: func(o *Orchestrator) {
				o.pendingJobs["job-1"] = JobInfo{ID: "job-1", WorkItem: WorkItem{ID: "job-1"}}
				o.completedJobs = []JobInfo{{ID: "job-2", WorkItem: WorkItem{ID: "job-2"}}}
			},
			assertion: func(t *testing.T, o *Orchestrator, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "already completed")
			},
		},
		{
			name:  "Success Rename and Cascade Dependencies",
			oldID: "job-1",
			newID: "new-job-1",
			setup: func(o *Orchestrator) {
				o.pendingJobs["job-1"] = JobInfo{ID: "job-1", WorkItem: WorkItem{ID: "job-1"}}
				o.pendingJobs["job-2"] = JobInfo{ID: "job-2", WorkItem: WorkItem{ID: "job-2", DependsOn: []string{"job-1", "other-job"}}}
			},
			assertion: func(t *testing.T, o *Orchestrator, err error) {
				assert.NoError(t, err)

				_, ok := o.pendingJobs["job-1"]
				assert.False(t, ok)

				newJob, ok := o.pendingJobs["new-job-1"]
				require.True(t, ok)
				assert.Equal(t, "new-job-1", newJob.ID)
				assert.Equal(t, "new-job-1", newJob.WorkItem.ID)

				depJob := o.pendingJobs["job-2"]
				assert.Equal(t, []string{"new-job-1", "other-job"}, depJob.WorkItem.DependsOn)
			},
		},
		{
			name:  "Success Rename With Timer",
			oldID: "job-1",
			newID: "new-job-1",
			setup: func(o *Orchestrator) {
				runAfter := time.Now().Add(1 * time.Hour)
				o.pendingJobs["job-1"] = JobInfo{ID: "job-1", WorkItem: WorkItem{ID: "job-1", RunAfter: runAfter}}
				o.delayTimers["job-1"] = time.AfterFunc(1*time.Hour, func() {})
			},
			assertion: func(t *testing.T, o *Orchestrator, err error) {
				assert.NoError(t, err)

				_, ok := o.delayTimers["job-1"]
				assert.False(t, ok)

				timer, ok := o.delayTimers["new-job-1"]
				require.True(t, ok)
				timer.Stop()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(nil, nil, time.Second)
			tt.setup(o)
			err := o.RenameJob(ctx, tt.oldID, tt.newID, nil)
			tt.assertion(t, o, err)
		})
	}
}

// This is just to test missing cases
func TestOrchestrator_RenameJob_DelayTimerExpired(t *testing.T) {
	ctx := context.Background()
	o := New(nil, nil, time.Second)

	// Create job with RunAfter = Time{}
	o.pendingJobs["job-1"] = JobInfo{ID: "job-1", WorkItem: WorkItem{ID: "job-1", RunAfter: time.Time{}}}
	o.delayTimers["job-1"] = time.AfterFunc(1*time.Hour, func() {})

	err := o.RenameJob(ctx, "job-1", "new-job-1", nil)
	assert.NoError(t, err)

	_, ok := o.delayTimers["new-job-1"]
	assert.False(t, ok) // Timer should not be rescheduled
}

// Missing test coverage block
func TestOrchestrator_RenameJob_DelayTimerCallback(t *testing.T) {
	ctx := context.Background()
	o := New(nil, nil, time.Second)

	runAfter := time.Now().Add(50 * time.Millisecond)
	o.pendingJobs["job-1"] = JobInfo{ID: "job-1", WorkItem: WorkItem{ID: "job-1", RunAfter: runAfter}}
	o.delayTimers["job-1"] = time.AfterFunc(1*time.Hour, func() {})

	err := o.RenameJob(ctx, "job-1", "new-job-1", nil)
	assert.NoError(t, err)

	require.Eventually(t, func() bool {
		o.mu.Lock()
		defer o.mu.Unlock()
		j, ok := o.pendingJobs["new-job-1"]
		if !ok {
			return false
		}

		_, timerExists := o.delayTimers["new-job-1"]
		return j.WorkItem.RunAfter.IsZero() && !timerExists
	}, 2*time.Second, 10*time.Millisecond)
}
