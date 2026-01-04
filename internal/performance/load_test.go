package performance

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/process-failed-successfully/recac/internal/workflow"
	"github.com/process-failed-successfully/recac/internal/jira"
)

// TestWorkflowUnderLoad simulates concurrent workflow executions to measure performance.
func TestWorkflowUnderLoad(t *testing.T) {
	const numWorkers = 100  // Simulate 100 concurrent workflows
	const numTickets = 1000 // Total tickets to process

	// Initialize workflow and Jira client (mock or real)
	wf := workflow.NewWorkflow()
	jiraClient := jira.NewClient() // Assume this is configured

	var wg sync.WaitGroup
	ticketsProcessed := 0
	errors := 0
	startTime := time.Now()

	// Worker function to process tickets
	worker := func(id int) {
		defer wg.Done()
		for {
			ticketID := fmt.Sprintf("MFLP-%d", rand.Intn(10000)+1000)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Simulate workflow execution
			err := wf.Execute(ctx, ticketID)
			if err != nil {
				errors++
				t.Logf("Worker %d failed for ticket %s: %v", id, ticketID, err)
				continue
			}

			ticketsProcessed++
			if ticketsProcessed >= numTickets {
				return
			}
		}
	}

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	t.Logf("Performance Results:")
	t.Logf("- Tickets Processed: %d", ticketsProcessed)
	t.Logf("- Errors: %d", errors)
	t.Logf("- Duration: %v", duration)
	t.Logf("- Throughput: %.2f tickets/sec", float64(ticketsProcessed)/duration.Seconds())

	// Assert performance criteria
	if float64(ticketsProcessed)/duration.Seconds() < 10 {
		t.Errorf("Throughput too low: %.2f tickets/sec (expected >= 10)", float64(ticketsProcessed)/duration.Seconds())
	}
	if float64(errors)/float64(ticketsProcessed) > 0.01 {
		t.Errorf("Error rate too high: %.2f%% (expected < 1%%)", float64(errors)/float64(ticketsProcessed)*100)
	}
}
