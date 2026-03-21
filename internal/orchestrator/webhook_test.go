package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestOrchestrator_JobWebhook_Success(t *testing.T) {
	var wg sync.WaitGroup
	var receivedPayload JobInfo
	var receivedCount int

	// 1. Create a mock HTTP server to receive the webhook
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer wg.Done()
		receivedCount++
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		err = json.Unmarshal(body, &receivedPayload)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// We expect 1 webhook call
	wg.Add(1)

	// 2. Setup Orchestrator with mock poller and spawner
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	// Poller returns no jobs
	mockPoller.On("Poll", mock.Anything, mock.Anything).Return([]WorkItem{}, nil)

	// Spawner immediately returns success
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	// 3. Submit a job with the webhook URL
	jobID := "webhook-job-1"
	item := WorkItem{
		ID:         jobID,
		Summary:    "Test Webhook Job",
		WebhookURL: ts.URL, // Use the mock server's URL
	}

	err := orch.SubmitJob(context.Background(), item, nil)
	assert.NoError(t, err)

	// Run orchestrator briefly so it processes the job
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go orch.Run(ctx, slog.Default())

	// Wait for webhook to be received with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for webhook")
	}

	// 4. Verify the webhook payload
	assert.Equal(t, 1, receivedCount, "Webhook should be called exactly once")
	assert.Equal(t, jobID, receivedPayload.ID, "Webhook payload should contain the correct Job ID")
	assert.Equal(t, "Completed", receivedPayload.Status, "Webhook payload should indicate Completed status")
	assert.Equal(t, "Test Webhook Job", receivedPayload.Summary)
}

func TestOrchestrator_JobWebhook_Failure(t *testing.T) {
	var wg sync.WaitGroup
	var receivedPayload JobInfo

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer wg.Done()
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	wg.Add(1)

	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	// Poller returns no jobs
	mockPoller.On("Poll", mock.Anything, mock.Anything).Return([]WorkItem{}, nil)

	// Spawner fails
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(assert.AnError)

	// Needs UpdateStatus to be mocked because of Spawner failure
	mockPoller.On("UpdateStatus", mock.Anything, mock.Anything, "Failed", mock.Anything).Return(nil)

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	jobID := "webhook-job-failed"
	item := WorkItem{
		ID:         jobID,
		Summary:    "Test Webhook Failing Job",
		WebhookURL: ts.URL,
	}

	err := orch.SubmitJob(context.Background(), item, nil)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go orch.Run(ctx, slog.Default())

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for webhook")
	}

	assert.Equal(t, jobID, receivedPayload.ID)
	assert.Equal(t, "Failed", receivedPayload.Status)
	assert.Contains(t, receivedPayload.Error, assert.AnError.Error())
}

func TestOrchestrator_JobWebhook_ForceComplete(t *testing.T) {
	var wg sync.WaitGroup
	var receivedPayload JobInfo

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer wg.Done()
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	wg.Add(1)

	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	jobID := "webhook-job-force-complete"
	item := WorkItem{
		ID:         jobID,
		Summary:    "Test Webhook Force Complete Job",
		WebhookURL: ts.URL,
		// Add an unmet dependency so it stays pending
		DependsOn:  []string{"non-existent-job"},
	}

	err := orch.SubmitJob(context.Background(), item, nil)
	assert.NoError(t, err)

	// Now force complete it
	err = orch.ForceCompleteJob(context.Background(), jobID, nil)
	assert.NoError(t, err)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for webhook")
	}

	assert.Equal(t, jobID, receivedPayload.ID)
	assert.Equal(t, "Completed", receivedPayload.Status)
}
