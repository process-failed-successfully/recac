package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStreamEvents_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		assert.True(t, ok)

		// 1. Connected Event
		fmt.Fprintf(w, "data: {\"event\": \"connected\", \"timestamp\": \"2023-10-01T12:00:00Z\", \"data\": {}}\n\n")
		flusher.Flush()

		// 2. Job Spawning Event
		fmt.Fprintf(w, "data: {\"event\": \"job_spawning\", \"timestamp\": \"2023-10-01T12:00:01Z\", \"data\": {\"id\": \"JOB-123\", \"summary\": \"Test Job\"}}\n\n")
		flusher.Flush()

		// 3. Job Completed Event
		fmt.Fprintf(w, "data: {\"event\": \"job_completed\", \"timestamp\": \"2023-10-01T12:00:02Z\", \"data\": {\"id\": \"JOB-123\", \"summary\": \"Test Job\", \"start_time\": \"2023-10-01T12:00:00Z\", \"end_time\": \"2023-10-01T12:00:02Z\"}}\n\n")
		flusher.Flush()

		// 4. Job Renamed Event
		fmt.Fprintf(w, "data: {\"event\": \"job_renamed\", \"timestamp\": \"2023-10-01T12:00:03Z\", \"data\": {\"old_id\": \"JOB-123\", \"new_id\": \"JOB-456\"}}\n\n")
		flusher.Flush()

		// 5. Job Progress Updated
		fmt.Fprintf(w, "data: {\"event\": \"job_progress_updated\", \"timestamp\": \"2023-10-01T12:00:04Z\", \"data\": {\"id\": \"JOB-456\", \"progress\": 50, \"status_message\": \"Halfway there\"}}\n\n")
		flusher.Flush()
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := streamEvents(ctx, server.URL)
	assert.NoError(t, err)

	output := out.String()

	// Just check if key components are in the output (ignoring exact color codes and timestamps)
	assert.Contains(t, output, "Listening for orchestrator events")
	assert.Contains(t, output, "connected")
	assert.Contains(t, output, "job_spawning")
	assert.Contains(t, output, "JOB-123")
	assert.Contains(t, output, "Test Job")
	assert.Contains(t, output, "job_completed")
	assert.Contains(t, output, "Duration: 2s")
	assert.Contains(t, output, "job_renamed")
	assert.Contains(t, output, "New ID: JOB-456")
	assert.Contains(t, output, "job_progress_updated")
	assert.Contains(t, output, "JOB-456")
	assert.Contains(t, output, "Progress: 50")
	assert.Contains(t, output, "Halfway there")
	assert.Contains(t, output, "Event stream closed")
}

func TestStreamEvents_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	err := streamEvents(context.Background(), "http://invalid-host:9999")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to orchestrator")
}

func TestStreamEvents_BadStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	err := streamEvents(context.Background(), server.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to subscribe to events: status 500")
}

func TestStreamEvents_BadData(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, _ := w.(http.Flusher)
		// Send malformed JSON
		fmt.Fprintf(w, "data: {bad json\n\n")
		flusher.Flush()
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	err := streamEvents(context.Background(), server.URL)
	assert.NoError(t, err) // It shouldn't crash, just log error and continue until stream closes

	output := out.String()
	assert.Contains(t, output, "Error parsing event data")
}
