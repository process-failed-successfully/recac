package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateEnvVars_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-123/env", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"env_vars": {"KEY": "VAL"}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateEnvVars(server.URL, "JOB-123", map[string]string{"KEY": "VAL"})

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Job JOB-123 environment variables updated to: KEY=VAL")
	assert.Equal(t, 0, exitCode)
}

func TestUpdateEnvVars_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-123/env", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`invalid env vars`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateEnvVars(server.URL, "JOB-123", map[string]string{"KEY": "VAL"})

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to update environment variables: invalid env vars")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkEnv(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/env" && r.Method == http.MethodPut {
			var body struct {
				EnvVars map[string]string `json:"env_vars"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			tag := r.URL.Query().Get("tag")
			match := r.URL.Query().Get("match")

			if tag == "backend" {
				require.Equal(t, "VAL1", body.EnvVars["KEY1"])
				w.WriteHeader(http.StatusOK)
				fmt.Fprintln(w, `{"updated": 2}`)
				return
			} else if match == "Fix" {
				require.Equal(t, "VAL2", body.EnvVars["KEY2"])
				w.WriteHeader(http.StatusOK)
				fmt.Fprintln(w, `{"updated": 1}`)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Capture stdout
	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() {
		stdout = oldStdout
	}()

	// 1. Test Tag
	updateBulkEnvVars(server.URL, "", "backend", map[string]string{"KEY1": "VAL1"})

	// 2. Test Match
	updateBulkEnvVars(server.URL, "Fix", "", map[string]string{"KEY2": "VAL2"})

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	require.Contains(t, output, "Successfully updated environment variables for 2 pending jobs.")
	require.Contains(t, output, "Successfully updated environment variables for 1 pending jobs.")
}
