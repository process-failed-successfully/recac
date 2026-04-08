package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildJobInteractive(t *testing.T) {
	// Backup original stdout, stdin, and exitFunc
	origStdout := stdout
	origStdin := stdin
	origExitFunc := exitFunc
	defer func() {
		stdout = origStdout
		stdin = origStdin
		exitFunc = origExitFunc
	}()

	// Mock exitFunc to prevent test failure on os.Exit
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}

	// Prepare mock input
	input := "Test Job Summary\n" + // Summary
		"https://github.com/org/repo\n" + // Repo URL
		"A test description\n" + // Description
		"JOB-1, JOB-2\n" + // Depends On
		"test, e2e\n" + // Tags
		"test-group\n" + // Concurrency Group
		"KEY1=VAL1\n" + // Env Var 1
		"KEY2=VAL2\n" + // Env Var 2
		"\n" + // Finish env vars
		"y\n" // Confirm submission

	stdin = bytes.NewBufferString(input)

	// Prepare mock output
	var outBuf bytes.Buffer
	stdout = &outBuf

	// Create a mock server to intercept the submission
	var submittedPayload []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs" && r.Method == http.MethodPost {
			submittedPayload, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("Job submitted successfully"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// Run the interactive builder
	buildJobInteractive(ts.URL, false)

	// Verify it didn't exit unexpectedly
	if exitCalled {
		t.Fatalf("buildJobInteractive called exitFunc unexpectedly. Output: %s", outBuf.String())
	}

	// Verify the submitted payload
	if len(submittedPayload) == 0 {
		t.Fatalf("No payload was submitted to the server")
	}

	var jobData map[string]interface{}
	if err := json.Unmarshal(submittedPayload, &jobData); err != nil {
		t.Fatalf("Failed to unmarshal submitted payload: %v", err)
	}

	// Validate fields
	if jobData["summary"] != "Test Job Summary" {
		t.Errorf("Expected summary 'Test Job Summary', got %v", jobData["summary"])
	}
	if jobData["repo_url"] != "https://github.com/org/repo" {
		t.Errorf("Expected repo_url 'https://github.com/org/repo', got %v", jobData["repo_url"])
	}
	if jobData["description"] != "A test description" {
		t.Errorf("Expected description 'A test description', got %v", jobData["description"])
	}
	if jobData["concurrency_group"] != "test-group" {
		t.Errorf("Expected concurrency_group 'test-group', got %v", jobData["concurrency_group"])
	}
	if jobData["cancel_in_progress"] != true {
		t.Errorf("Expected cancel_in_progress true, got %v", jobData["cancel_in_progress"])
	}

	// Validate DependsOn
	dependsOn, ok := jobData["depends_on"].([]interface{})
	if !ok || len(dependsOn) != 2 || dependsOn[0] != "JOB-1" || dependsOn[1] != "JOB-2" {
		t.Errorf("Expected depends_on ['JOB-1', 'JOB-2'], got %v", jobData["depends_on"])
	}

	// Validate Tags
	tags, ok := jobData["tags"].([]interface{})
	if !ok || len(tags) != 2 || tags[0] != "test" || tags[1] != "e2e" {
		t.Errorf("Expected tags ['test', 'e2e'], got %v", jobData["tags"])
	}

	// Validate EnvVars
	envVars, ok := jobData["env_vars"].(map[string]interface{})
	if !ok || len(envVars) != 2 || envVars["KEY1"] != "VAL1" || envVars["KEY2"] != "VAL2" {
		t.Errorf("Expected env_vars {'KEY1': 'VAL1', 'KEY2': 'VAL2'}, got %v", jobData["env_vars"])
	}

	// Verify it auto-generated an ID
	id, ok := jobData["id"].(string)
	if !ok || !strings.HasPrefix(id, "adhoc-") || len(id) < 8 {
		t.Errorf("Expected valid auto-generated id, got %v", jobData["id"])
	}
}

func TestBuildJobInteractive_Cancel(t *testing.T) {
	// Backup original stdout, stdin, and exitFunc
	origStdout := stdout
	origStdin := stdin
	origExitFunc := exitFunc
	defer func() {
		stdout = origStdout
		stdin = origStdin
		exitFunc = origExitFunc
	}()

	// Mock exitFunc
	exitFunc = func(code int) {}

	// Prepare mock input with "n" for confirmation
	input := "Test\n" + // Summary
		"url\n" + // Repo
		"\n" + // Desc
		"\n" + // Deps
		"\n" + // Tags
		"\n" + // Group
		"\n" + // Finish env vars
		"n\n" // Cancel submission

	stdin = bytes.NewBufferString(input)
	var outBuf bytes.Buffer
	stdout = &outBuf

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("Server should not be hit on cancellation")
	}))
	defer ts.Close()

	buildJobInteractive(ts.URL, false)

	outStr := outBuf.String()
	if !strings.Contains(outStr, "Job submission cancelled.") {
		t.Errorf("Expected cancellation message, got: %s", outStr)
	}
}
