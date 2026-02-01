package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLoadCommand(t *testing.T) {
	// Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Millisecond) // Simulate some latency
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	}))
	defer ts.Close()

	t.Run("Basic Request Limit", func(t *testing.T) {
		// Setup globals
		loadURL = ts.URL
		loadConcurrency = 2
		loadRequests = 10
		loadDuration = 0

		buf := new(bytes.Buffer)
		cmd := loadCmd
		cmd.SetOut(buf)

		// Flags haven't been parsed, so Changed("requests") is false.
		// But loadDuration is 0, so the override logic doesn't trigger.

		err := runLoadTest(cmd, []string{})
		if err != nil {
			t.Fatalf("runLoadTest failed: %v", err)
		}

		output := buf.String()
		// Tabwriter uses tabs, so match carefully or use strings.Contains
		if !strings.Contains(output, "Total Requests:") {
			t.Errorf("Output missing Total Requests")
		}
		// Check for "10" (assuming spacing is tabbed)
		// "Total Requests: \t10"
		if !strings.Contains(output, "10") {
			t.Errorf("Expected 10 requests, got output:\n%s", output)
		}
		if !strings.Contains(output, "100.00%") {
			t.Errorf("Expected 100%% success, got output:\n%s", output)
		}
	})

	t.Run("Duration Limit", func(t *testing.T) {
		loadURL = ts.URL
		loadConcurrency = 5
		loadRequests = 100 // This will be overridden because duration > 0 and Changed=false
		loadDuration = 50 * time.Millisecond

		buf := new(bytes.Buffer)
		cmd := loadCmd
		cmd.SetOut(buf)

		err := runLoadTest(cmd, []string{})
		if err != nil {
			t.Fatalf("runLoadTest failed: %v", err)
		}

		output := buf.String()
		if strings.Contains(output, "Total Requests: \t0") {
			t.Errorf("Should have run some requests")
		}
		// Should verify it didn't run forever
	})
}
