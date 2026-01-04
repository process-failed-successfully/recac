package orchestrator

import (
	"context"
	"log"
	"time"
)

// RunJob runs a job and handles failures appropriately
func (o *Orchestrator) RunJob(ctx context.Context, job *Job) error {
	log.Printf("Starting job %s", job.ID)

	// Your existing job execution logic here
	// ...

	// Simulate job execution (replace with actual logic)
	time.Sleep(1 * time.Second)

	// For demonstration, let's simulate a failure condition
	// In real implementation, this would be based on actual job execution
	if job.Spec.Command == "fail" {
		return fmt.Errorf("simulated job failure")
	}

	return nil
}

// RunJobWithFailureHandling runs a job with proper failure handling
func (o *Orchestrator) RunJobWithFailureHandling(ctx context.Context, job *Job) error {
	// Mark job as started
	job.Status = JobStatusRunning
	job.StartedAt = time.Now()

	if err := o.db.SaveJob(job); err != nil {
		return fmt.Errorf("failed to save job start state: %w", err)
	}

	// Run the job
	err := o.RunJob(ctx, job)

	if err != nil {
		// Handle failure
		return o.handleJobFailure(ctx, job, err)
	}

	// Job succeeded
	job.Status = JobStatusCompleted
	job.FinishedAt = time.Now()

	if err := o.db.SaveJob(job); err != nil {
		return fmt.Errorf("failed to save job completion state: %w", err)
	}

	return nil
}

// handleJobFailure handles job failures and updates Jira accordingly
func (o *Orchestrator) handleJobFailure(ctx context.Context, job *Job, err error) error {
	// Log the failure
	log.Printf("Job %s failed: %v", job.ID, err)

	// Update Jira ticket with failure information
	if job.JiraTicket != "" {
		if err := o.updateJiraOnFailure(ctx, job, err); err != nil {
			log.Printf("Failed to update Jira for job %s: %v", job.ID, err)
			// Don't return this error as we want to continue with other failure handling
		}
	}

	// Mark job as failed
	job.Status = JobStatusFailed
	job.Error = err.Error()
	job.FinishedAt = time.Now()

	// Save the updated job state
	if err := o.db.SaveJob(job); err != nil {
		return fmt.Errorf("failed to save job state: %w", err)
	}

	return nil
}

// updateJiraOnFailure updates the Jira ticket with failure information
func (o *Orchestrator) updateJiraOnFailure(ctx context.Context, job *Job, err error) error {
	// Create a detailed failure comment
	failureComment := fmt.Sprintf("Job failed with error: %v\n\n" +
		"Job ID: %s\n" +
		"Started at: %s\n" +
		"Failed at: %s",
		err,
		job.ID,
		job.StartedAt.Format(time.RFC3339),
		time.Now().Format(time.RFC3339))

	// Add comment to Jira ticket
	if err := o.jiraClient.AddComment(ctx, job.JiraTicket, failureComment); err != nil {
		return fmt.Errorf("failed to add comment to Jira ticket: %w", err)
	}

	// Transition ticket to appropriate failure state
	// Try common failure states
	failureStates := []string{"Failed", "Blocked", "Needs Review", "Error"}

	for _, state := range failureStates {
		if err := o.jiraClient.SmartTransition(ctx, job.JiraTicket, state); err == nil {
			log.Printf("Transitioned Jira ticket %s to %s state", job.JiraTicket, state)
			return nil
		}
	}

	// If no specific failure state is available, just log it
	log.Printf("No suitable failure transition found for Jira ticket %s", job.JiraTicket)
	return nil
}
