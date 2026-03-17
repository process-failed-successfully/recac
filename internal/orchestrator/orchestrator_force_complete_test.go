package orchestrator

import (
	"context"
	"testing"
	"time"
)

func TestForceCompleteJob_Pending(t *testing.T) {
	o := New(&mockPoller{}, &mockSpawner{}, time.Second)

	// Create a pending job
	job := JobInfo{
		ID:        "job-pending",
		Summary:   "Pending Job",
		Status:    "Pending",
		StartTime: time.Now(),
		WorkItem: WorkItem{
			ID: "job-pending",
		},
	}

	o.mu.Lock()
	o.pendingJobs["job-pending"] = job
	o.mu.Unlock()

	err := o.ForceCompleteJob(context.Background(), "job-pending", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	if _, exists := o.pendingJobs["job-pending"]; exists {
		t.Errorf("expected job to be removed from pending jobs")
	}

	found := false
	for _, cJob := range o.completedJobs {
		if cJob.ID == "job-pending" {
			found = true
			if cJob.Status != "Completed" {
				t.Errorf("expected status Completed, got %s", cJob.Status)
			}
			break
		}
	}

	if !found {
		t.Errorf("expected job to be in history")
	}
}

func TestForceCompleteJob_Active(t *testing.T) {
	mockSp := &mockSpawner{}
	o := New(&mockPoller{}, mockSp, time.Second)

	// Create an active job
	job := JobInfo{
		ID:        "job-active",
		Summary:   "Active Job",
		Status:    "Running",
		StartTime: time.Now(),
		WorkItem: WorkItem{
			ID: "job-active",
		},
	}

	o.mu.Lock()
	o.activeJobs["job-active"] = job
	o.activeSpawns = 1
	o.mu.Unlock()

	err := o.ForceCompleteJob(context.Background(), "job-active", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	o.mu.RLock()

	if _, exists := o.activeJobs["job-active"]; exists {
		t.Errorf("expected job to be removed from active jobs")
	}

	if o.activeSpawns != 0 {
		t.Errorf("expected active spawns to be 0, got %d", o.activeSpawns)
	}

	found := false
	for _, cJob := range o.completedJobs {
		if cJob.ID == "job-active" {
			found = true
			if cJob.Status != "Completed" {
				t.Errorf("expected status Completed, got %s", cJob.Status)
			}
			break
		}
	}
	o.mu.RUnlock()

	if !found {
		t.Errorf("expected job to be in history")
	}
}

func TestForceCompleteJob_Failed(t *testing.T) {
	o := New(&mockPoller{}, &mockSpawner{}, time.Second)

	// Create a failed job in history
	job := JobInfo{
		ID:        "job-failed",
		Summary:   "Failed Job",
		Status:    "Failed",
		StartTime: time.Now(),
		Error:     "some error",
		WorkItem: WorkItem{
			ID: "job-failed",
		},
	}

	o.mu.Lock()
	o.completedJobs = append(o.completedJobs, job)
	o.mu.Unlock()

	err := o.ForceCompleteJob(context.Background(), "job-failed", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	found := false
	failedCount := 0
	for _, cJob := range o.completedJobs {
		if cJob.ID == "job-failed" {
			failedCount++
			found = true
			if cJob.Status != "Completed" {
				t.Errorf("expected status Completed, got %s", cJob.Status)
			}
			if cJob.Error != "" {
				t.Errorf("expected error to be cleared")
			}
		}
	}

	if !found {
		t.Errorf("expected job to be in history")
	}

	if failedCount > 1 {
		t.Errorf("expected job to only appear once in history")
	}
}

func TestForceCompleteJobsByTag(t *testing.T) {
	o := New(&mockPoller{}, &mockSpawner{}, time.Second)

	o.mu.Lock()
	o.pendingJobs["j1"] = JobInfo{ID: "j1", Status: "Pending", WorkItem: WorkItem{ID: "j1", Tags: []string{"tag1"}}}
	o.activeJobs["j2"] = JobInfo{ID: "j2", Status: "Running", WorkItem: WorkItem{ID: "j2", Tags: []string{"tag1"}}}
	o.completedJobs = append(o.completedJobs, JobInfo{ID: "j3", Status: "Failed", WorkItem: WorkItem{ID: "j3", Tags: []string{"tag1"}}})
	o.pendingJobs["j4"] = JobInfo{ID: "j4", Status: "Pending", WorkItem: WorkItem{ID: "j4", Tags: []string{"other"}}}
	o.mu.Unlock()

	count, err := o.ForceCompleteJobsByTag(context.Background(), "tag1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 jobs to be force completed, got %d", count)
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	if len(o.completedJobs) != 3 { // j1, j2, and j3 should be completed
		t.Errorf("expected 3 completed jobs, got %d", len(o.completedJobs))
	}

	for _, cJob := range o.completedJobs {
		if cJob.Status != "Completed" {
			t.Errorf("expected status Completed, got %s for job %s", cJob.Status, cJob.ID)
		}
	}
}

func TestForceCompleteJobsByMatch(t *testing.T) {
	o := New(&mockPoller{}, &mockSpawner{}, time.Second)

	o.mu.Lock()
	o.pendingJobs["j1"] = JobInfo{ID: "j1", Summary: "fix bug 1", Status: "Pending", WorkItem: WorkItem{ID: "j1"}}
	o.activeJobs["j2"] = JobInfo{ID: "j2", Summary: "fix bug 2", Status: "Running", WorkItem: WorkItem{ID: "j2"}}
	o.completedJobs = append(o.completedJobs, JobInfo{ID: "j3", Summary: "feature", Error: "could not fix bug", Status: "Failed", WorkItem: WorkItem{ID: "j3"}})
	o.pendingJobs["j4"] = JobInfo{ID: "j4", Summary: "doc update", Status: "Pending", WorkItem: WorkItem{ID: "j4"}}
	o.mu.Unlock()

	count, err := o.ForceCompleteJobsByMatch(context.Background(), "fix bug", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 jobs to be force completed, got %d", count)
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	for _, cJob := range o.completedJobs {
		if cJob.ID == "j1" || cJob.ID == "j2" || cJob.ID == "j3" {
			if cJob.Status != "Completed" {
				t.Errorf("expected status Completed, got %s for job %s", cJob.Status, cJob.ID)
			}
		}
	}
}
