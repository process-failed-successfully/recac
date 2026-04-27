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

func TestUpdateBulkTags_DecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/tags", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkTags(server.URL, "", "", []string{"test"})

	assert.Contains(t, buf.String(), "Failed to decode response")
	assert.Equal(t, 1, exitCode)
}

func TestAddTags(t *testing.T) {
	tests := []struct {
		name           string
		handler        http.HandlerFunc
		useRealServer  bool
		expectedOutput string
		expectExit     bool
	}{
		{
			name: "Success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "PUT", r.Method)
				assert.Equal(t, "/jobs/TEST-123/tags/add", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				fmt.Fprintln(w, `{"message": "tags added successfully"}`)
			},
			useRealServer:  true,
			expectedOutput: "Job TEST-123 tags added: tag1",
			expectExit:     false,
		},
		{
			name: "Error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintln(w, "job TEST-123 not found")
			},
			useRealServer:  true,
			expectedOutput: "Failed to add tags",
			expectExit:     true,
		},
		{
			name:           "Connection Error",
			useRealServer:  false,
			expectedOutput: "Failed to connect to orchestrator",
			expectExit:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var urlStr string
			if tt.useRealServer {
				server := httptest.NewServer(tt.handler)
				defer server.Close()
				urlStr = server.URL
			} else {
				urlStr = "http://localhost:0"
			}

			var buf bytes.Buffer
			oldStdout := stdout
			stdout = &buf
			defer func() { stdout = oldStdout }()

			exitCalled := false
			oldExit := exitFunc
			exitFunc = func(code int) { exitCalled = true }
			defer func() { exitFunc = oldExit }()

			viper.Reset()
			defer viper.Reset()

			addTags(urlStr, "TEST-123", []string{"tag1"})

			output := buf.String()
			assert.Contains(t, output, tt.expectedOutput)
			assert.Equal(t, tt.expectExit, exitCalled)
		})
	}
}

func TestRemoveTags(t *testing.T) {
	tests := []struct {
		name           string
		handler        http.HandlerFunc
		useRealServer  bool
		expectedOutput string
		expectExit     bool
	}{
		{
			name: "Success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "PUT", r.Method)
				assert.Equal(t, "/jobs/TEST-123/tags/remove", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				fmt.Fprintln(w, `{"message": "tags removed successfully"}`)
			},
			useRealServer:  true,
			expectedOutput: "Job TEST-123 tags removed: tag1",
			expectExit:     false,
		},
		{
			name: "Error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintln(w, "job TEST-123 not found")
			},
			useRealServer:  true,
			expectedOutput: "Failed to remove tags",
			expectExit:     true,
		},
		{
			name:           "Connection Error",
			useRealServer:  false,
			expectedOutput: "Failed to connect to orchestrator",
			expectExit:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var urlStr string
			if tt.useRealServer {
				server := httptest.NewServer(tt.handler)
				defer server.Close()
				urlStr = server.URL
			} else {
				urlStr = "http://localhost:0"
			}

			var buf bytes.Buffer
			oldStdout := stdout
			stdout = &buf
			defer func() { stdout = oldStdout }()

			exitCalled := false
			oldExit := exitFunc
			exitFunc = func(code int) { exitCalled = true }
			defer func() { exitFunc = oldExit }()

			viper.Reset()
			defer viper.Reset()

			removeTags(urlStr, "TEST-123", []string{"tag1"})

			output := buf.String()
			assert.Contains(t, output, tt.expectedOutput)
			assert.Equal(t, tt.expectExit, exitCalled)
		})
	}
}

func TestAddBulkTags(t *testing.T) {
	tests := []struct {
		name           string
		handler        http.HandlerFunc
		useRealServer  bool
		match          string
		tag            string
		expectedOutput string
		expectExit     bool
	}{
		{
			name: "AddTagsByTag_Success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/jobs/tags/add", r.URL.Path)
				assert.Equal(t, "tag1", r.URL.Query().Get("tag"))
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"updated": 2}`))
			},
			useRealServer:  true,
			tag:            "tag1",
			expectedOutput: "Successfully added tags for 2 pending jobs",
			expectExit:     false,
		},
		{
			name: "AddTagsByMatch_Success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/jobs/tags/add", r.URL.Path)
				assert.Equal(t, "regex", r.URL.Query().Get("match"))
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"updated": 1}`))
			},
			useRealServer:  true,
			match:          "regex",
			expectedOutput: "Successfully added tags for 1 pending jobs",
			expectExit:     false,
		},
		{
			name: "ServerError",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "server error", http.StatusInternalServerError)
			},
			useRealServer:  true,
			match:          "regex",
			expectedOutput: "Failed to add bulk tags:",
			expectExit:     true,
		},
		{
			name:           "ConnectionError",
			useRealServer:  false,
			match:          "regex",
			expectedOutput: "Failed to connect to orchestrator",
			expectExit:     true,
		},
		{
			name: "DecodeError",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`invalid json`))
			},
			useRealServer:  true,
			expectedOutput: "Failed to decode response",
			expectExit:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var urlStr string
			if tt.useRealServer {
				mux := http.NewServeMux()
				mux.HandleFunc("/jobs/tags/add", tt.handler)
				server := httptest.NewServer(mux)
				defer server.Close()
				urlStr = server.URL
			} else {
				urlStr = "http://localhost:0"
			}

			var buf bytes.Buffer
			oldStdout := stdout
			stdout = &buf
			defer func() { stdout = oldStdout }()

			exitCalled := false
			oldExit := exitFunc
			exitFunc = func(code int) { exitCalled = true }
			defer func() { exitFunc = oldExit }()

			addBulkTags(urlStr, tt.match, tt.tag, []string{"newtag"})

			output := buf.String()
			assert.Contains(t, output, tt.expectedOutput)
			assert.Equal(t, tt.expectExit, exitCalled)
		})
	}
}

func TestRemoveBulkTags(t *testing.T) {
	tests := []struct {
		name           string
		handler        http.HandlerFunc
		useRealServer  bool
		match          string
		tag            string
		expectedOutput string
		expectExit     bool
	}{
		{
			name: "RemoveTagsByTag_Success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/jobs/tags/remove", r.URL.Path)
				assert.Equal(t, "tag1", r.URL.Query().Get("tag"))
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"updated": 2}`))
			},
			useRealServer:  true,
			tag:            "tag1",
			expectedOutput: "Successfully removed tags for 2 pending jobs",
			expectExit:     false,
		},
		{
			name: "RemoveTagsByMatch_Success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/jobs/tags/remove", r.URL.Path)
				assert.Equal(t, "regex", r.URL.Query().Get("match"))
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"updated": 1}`))
			},
			useRealServer:  true,
			match:          "regex",
			expectedOutput: "Successfully removed tags for 1 pending jobs",
			expectExit:     false,
		},
		{
			name: "ServerError",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "server error", http.StatusInternalServerError)
			},
			useRealServer:  true,
			match:          "regex",
			expectedOutput: "Failed to remove bulk tags:",
			expectExit:     true,
		},
		{
			name:           "ConnectionError",
			useRealServer:  false,
			match:          "regex",
			expectedOutput: "Failed to connect to orchestrator",
			expectExit:     true,
		},
		{
			name: "DecodeError",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`invalid json`))
			},
			useRealServer:  true,
			expectedOutput: "Failed to decode response",
			expectExit:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var urlStr string
			if tt.useRealServer {
				mux := http.NewServeMux()
				mux.HandleFunc("/jobs/tags/remove", tt.handler)
				server := httptest.NewServer(mux)
				defer server.Close()
				urlStr = server.URL
			} else {
				urlStr = "http://localhost:0"
			}

			var buf bytes.Buffer
			oldStdout := stdout
			stdout = &buf
			defer func() { stdout = oldStdout }()

			exitCalled := false
			oldExit := exitFunc
			exitFunc = func(code int) { exitCalled = true }
			defer func() { exitFunc = oldExit }()

			removeBulkTags(urlStr, tt.match, tt.tag, []string{"newtag"})

			output := buf.String()
			assert.Contains(t, output, tt.expectedOutput)
			assert.Equal(t, tt.expectExit, exitCalled)
		})
	}
}
