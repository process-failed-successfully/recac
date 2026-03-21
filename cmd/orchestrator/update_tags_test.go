package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestUpdateTags(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/jobs/TEST-123/tags", r.URL.Path)

		var body bytes.Buffer
		body.ReadFrom(r.Body)
		assert.Contains(t, body.String(), `"tags":["tag1","tag2"]`)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"message": "tags updated successfully"}`)
	}))
	defer server.Close()

	// Capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitFunc = func(code int) {}
	defer func() { exitFunc = oldExit }()

	// Reset viper
	viper.Reset()
	defer viper.Reset()

	// Call function
	updateTags(server.URL, "TEST-123", []string{"tag1", "tag2"})

	// Verify output
	output := buf.String()
	assert.Contains(t, output, "Job TEST-123 tags updated to: tag1, tag2")
}

func TestUpdateTags_Error(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, "job TEST-123 not found")
	}))
	defer server.Close()

	// Capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	// Reset viper
	viper.Reset()
	defer viper.Reset()

	// Call function
	updateTags(server.URL, "TEST-123", []string{"tag1"})

	// Verify output
	output := buf.String()
	assert.Contains(t, output, "Failed to update tags")
	assert.Contains(t, output, "job TEST-123 not found")
	assert.True(t, exitCalled)
}

func TestUpdateTags_ConnectionError(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	updateTags("http://localhost:0", "TEST-123", []string{"tag1"})

	output := buf.String()
	assert.Contains(t, output, "Failed to connect to orchestrator")
	assert.True(t, exitCalled)
}

func TestUpdateBulkTags(t *testing.T) {
	t.Run("UpdateTagsByTag_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/jobs/tags", r.URL.Path)
			assert.Equal(t, "tag1", r.URL.Query().Get("tag"))
			assert.Equal(t, http.MethodPut, r.Method)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"updated": 2}`))
		}))
		defer server.Close()

		var stdoutBuf bytes.Buffer
		stdout = &stdoutBuf

		oldExit := exitFunc
		defer func() { exitFunc = oldExit }()
		exitFunc = func(code int) {
			t.Fatalf("unexpected exit: %d", code)
		}

		updateBulkTags(server.URL, "", "tag1", []string{"newtag"})

		output := stdoutBuf.String()
		assert.Contains(t, output, "Successfully updated tags for 2 pending jobs")
	})

	t.Run("UpdateTagsByMatch_Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/jobs/tags", r.URL.Path)
			assert.Equal(t, "regex", r.URL.Query().Get("match"))
			assert.Equal(t, http.MethodPut, r.Method)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"updated": 1}`))
		}))
		defer server.Close()

		var stdoutBuf bytes.Buffer
		stdout = &stdoutBuf

		oldExit := exitFunc
		defer func() { exitFunc = oldExit }()
		exitFunc = func(code int) {
			t.Fatalf("unexpected exit: %d", code)
		}

		updateBulkTags(server.URL, "regex", "", []string{"newtag"})

		output := stdoutBuf.String()
		assert.Contains(t, output, "Successfully updated tags for 1 pending jobs")
	})

	t.Run("ServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "server error", http.StatusInternalServerError)
		}))
		defer server.Close()

		var stdoutBuf bytes.Buffer
		stdout = &stdoutBuf

		exited := false
		oldExit := exitFunc
		defer func() { exitFunc = oldExit }()
		exitFunc = func(code int) {
			exited = true
			assert.Equal(t, 1, code)
		}

		updateBulkTags(server.URL, "regex", "", []string{"newtag"})

		assert.True(t, exited)
		output := stdoutBuf.String()
		assert.Contains(t, output, "Failed to update bulk tags:")
	})

	t.Run("ConnectionError", func(t *testing.T) {
		var stdoutBuf bytes.Buffer
		oldStdout := stdout
		stdout = &stdoutBuf
		defer func() { stdout = oldStdout }()

		exited := false
		oldExit := exitFunc
		defer func() { exitFunc = oldExit }()
		exitFunc = func(code int) {
			exited = true
			assert.Equal(t, 1, code)
		}

		updateBulkTags("http://localhost:0", "regex", "", []string{"newtag"})

		assert.True(t, exited)
		output := stdoutBuf.String()
		assert.Contains(t, output, "Failed to connect to orchestrator")
	})
}
