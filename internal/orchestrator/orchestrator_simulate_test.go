package orchestrator

import (
	"io/ioutil"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOrchestrator_Simulate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(ioutil.Discard, nil))
	o := &Orchestrator{
		activeJobs:        make(map[string]JobInfo),
		pendingJobs:       make(map[string]JobInfo),
		completedJobs:     []JobInfo{},
		MaxConcurrentJobs: 2,
	}

	t.Run("Empty State", func(t *testing.T) {
		report := o.Simulate(logger)
		assert.Equal(t, 0.0, report.EstimatedTotalTimeMs)
		assert.Equal(t, 0, report.JobsProcessed)
		assert.Equal(t, 0, report.Deadlocks)
	})

	t.Run("Single Pending Job", func(t *testing.T) {
		o.pendingJobs["job1"] = JobInfo{
			ID: "job1",
			WorkItem: WorkItem{
				Priority: 10,
			},
		}

		report := o.Simulate(logger)
		assert.Equal(t, 60000.0, report.EstimatedTotalTimeMs) // Default mean
		assert.Equal(t, 1, report.JobsProcessed)
		assert.Equal(t, 1, report.TotalJobs)
		assert.Equal(t, "job1", report.FinalBottleneckJob)
		assert.Equal(t, 0, report.Deadlocks)

		delete(o.pendingJobs, "job1") // Cleanup
	})

	t.Run("Dependencies and Concurrency", func(t *testing.T) {
		// job1 (60s) -> job3 (60s)
		// job2 (60s)
		// MaxConcurrentJobs = 2.
		// job1 and job2 start at 0s, both finish at 60s.
		// job3 starts at 60s, finishes at 120s.
		o.pendingJobs["job1"] = JobInfo{ID: "job1"}
		o.pendingJobs["job2"] = JobInfo{ID: "job2"}
		o.pendingJobs["job3"] = JobInfo{ID: "job3", WorkItem: WorkItem{DependsOn: []string{"job1"}}}

		report := o.Simulate(logger)
		assert.Equal(t, 120000.0, report.EstimatedTotalTimeMs)
		assert.Equal(t, 3, report.JobsProcessed)
		assert.Equal(t, 0, report.Deadlocks)

		delete(o.pendingJobs, "job1")
		delete(o.pendingJobs, "job2")
		delete(o.pendingJobs, "job3")
	})

	t.Run("Deadlock Detection", func(t *testing.T) {
		// A depends on B, B depends on A
		o.pendingJobs["jobA"] = JobInfo{ID: "jobA", WorkItem: WorkItem{DependsOn: []string{"jobB"}}}
		o.pendingJobs["jobB"] = JobInfo{ID: "jobB", WorkItem: WorkItem{DependsOn: []string{"jobA"}}}

		report := o.Simulate(logger)
		assert.Equal(t, 0, report.JobsProcessed)
		assert.Equal(t, 2, report.Deadlocks)
		assert.Equal(t, 2, report.TotalJobs)

		delete(o.pendingJobs, "jobA")
		delete(o.pendingJobs, "jobB")
	})

	t.Run("Averages calculation", func(t *testing.T) {
		o.completedJobs = append(o.completedJobs, JobInfo{
			ID:        "comp1",
			StartTime: time.Now(),
			EndTime:   time.Now().Add(10 * time.Second), // 10s
		})

		o.pendingJobs["job1"] = JobInfo{ID: "job1"}

		report := o.Simulate(logger)
		assert.Equal(t, 10000.0, report.EstimatedTotalTimeMs) // Mean should now be 10s

		delete(o.pendingJobs, "job1")
		o.completedJobs = []JobInfo{} // Cleanup
	})
}
func TestSimulatePipeline(t *testing.T) {
	o := New(nil, nil, 1*time.Second)

	// Add some history
	t1 := time.Now()
	o.completedJobs = []JobInfo{
		{
			ID: "hist-1",
			WorkItem: WorkItem{
				Summary: "Build specific app",
				Tags:    []string{"build"},
			},
			StartTime: t1,
			EndTime:   t1.Add(120 * time.Second), // avg for 'Build specific app' = 120s
		},
		{
			ID: "hist-2",
			WorkItem: WorkItem{
				Summary: "Other build",
				Tags:    []string{"build"},
			},
			StartTime: t1,
			EndTime:   t1.Add(80 * time.Second), // avg for tag 'build' = (120+80)/2 = 100s
		},
	}

	pipelineItems := []WorkItem{
		{
			ID:      "job-1",
			Summary: "Build specific app", // Should use exact summary avg: 120s
			Tags:    []string{"build"},
		},
		{
			ID:        "job-2",
			Summary:   "New build app", // Should fallback to tag 'build' avg: 100s
			Tags:      []string{"build"},
			DependsOn: []string{"job-1"},
		},
		{
			ID:        "job-3",
			Summary:   "Test app", // No tag, no summary. Should use globalMean = (120+80)/2 = 100s
			DependsOn: []string{"job-2"},
		},
	}

	report := o.SimulatePipeline(pipelineItems, nil)

	// Since maxWorkers=0 defaults to MaxInt32, they run sequentially due to DependsOn.
	// 120 + 100 + 100 = 320s = 320000ms
	assert.Equal(t, float64(320000), report.EstimatedTotalTimeMs)
	assert.Equal(t, 3, report.JobsProcessed)
	assert.Equal(t, 0, report.Deadlocks)
	assert.Equal(t, "job-3", report.FinalBottleneckJob)
}
