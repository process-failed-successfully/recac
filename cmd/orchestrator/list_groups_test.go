package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListGroups_TableFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/groups", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"name": "group-1", "active_jobs": 2, "pending_jobs": 5, "paused": false},
			{"name": "group-2", "active_jobs": 0, "pending_jobs": 0, "paused": true}
		]`))
	}))
	defer server.Close()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	listGroups(server.URL, "table")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "Concurrency Groups (2)")
	assert.Contains(t, output, "group-1")
	assert.Contains(t, output, "group-2")
	assert.Contains(t, output, "true")
	assert.Contains(t, output, "false")
	assert.Contains(t, output, "5")
}

func TestListGroups_JSONFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/groups", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"name": "json-group", "active_jobs": 1, "pending_jobs": 0, "paused": false}]`))
	}))
	defer server.Close()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	listGroups(server.URL, "json")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, `"name": "json-group"`)
	assert.Contains(t, output, `"active_jobs": 1`)
	assert.NotContains(t, output, "Concurrency Groups")
}
