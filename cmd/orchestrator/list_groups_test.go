package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListGroups(t *testing.T) {
	tests := []struct {
		name           string
		format         string
		url            string
		handler        http.HandlerFunc
		expectedOutput string
		expectedExit   int
	}{
		{
			name:   "TableFormat",
			format: "table",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/groups", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`[
					{"name": "group-1", "active_jobs": 2, "pending_jobs": 5, "paused": false},
					{"name": "group-2", "active_jobs": 0, "pending_jobs": 0, "paused": true}
				]`))
			},
			expectedOutput: "Concurrency Groups (2)",
			expectedExit:   0,
		},
		{
			name:   "JSONFormat",
			format: "json",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/groups", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`[{"name": "json-group", "active_jobs": 1, "pending_jobs": 0, "paused": false}]`))
			},
			expectedOutput: `"name": "json-group"`,
			expectedExit:   0,
		},
		{
			name:   "Empty",
			format: "table",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/groups", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`[]`))
			},
			expectedOutput: "No concurrency groups found",
			expectedExit:   0,
		},
		{
			name:   "ParseError",
			format: "table",
			url:    "://invalid-url",
			expectedOutput: "Failed to parse host URL",
			expectedExit:   1,
		},
		{
			name:   "ConnectionError",
			format: "table",
			url:    "http://invalid-url",
			expectedOutput: "Failed to connect to orchestrator",
			expectedExit:   1,
		},
		{
			name:   "ErrorResponse",
			format: "table",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/groups", r.URL.Path)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`Internal Server Error`))
			},
			expectedOutput: "Failed to fetch groups: status 500",
			expectedExit:   1,
		},
		{
			name:   "DecodeError",
			format: "table",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/groups", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`invalid json`))
			},
			expectedOutput: "Failed to decode response",
			expectedExit:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var serverURL string
			if tt.handler != nil {
				server := httptest.NewServer(tt.handler)
				defer server.Close()
				serverURL = server.URL
			} else {
				serverURL = tt.url
			}

			oldStdout := stdout
			var buf bytes.Buffer
			stdout = &buf
			defer func() { stdout = oldStdout }()

			oldExit := exitFunc
			var exitCode int
			exitFunc = func(code int) { exitCode = code }
			defer func() { exitFunc = oldExit }()

			listGroups(serverURL, tt.format)

			assert.Contains(t, buf.String(), tt.expectedOutput)
			assert.Equal(t, tt.expectedExit, exitCode)
		})
	}
}
