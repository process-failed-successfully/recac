package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealBulkJobs_Tag(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/heal/bulk", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "broken", r.URL.Query().Get("tag"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"healed": 3}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	stdout = w

	healBulkJobs(server.URL, "", "broken")

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Successfully healed 3 failed jobs.")
}

func TestHealBulkJobs_Match(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/heal/bulk", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "timeout", r.URL.Query().Get("match"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"healed": 2}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	stdout = w

	healBulkJobs(server.URL, "timeout", "")

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Successfully healed 2 failed jobs.")
}
