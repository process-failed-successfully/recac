package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubmitMatrixJob(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/jobs/matrix", r.URL.Path)
			assert.Equal(t, "POST", r.Method)
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"submitted": []string{"job-1", "job-2"},
				"errors":    []string{},
			})
		}))
		defer server.Close()

		tmpFile, _ := os.CreateTemp("", "matrix-*.json")
		defer os.Remove(tmpFile.Name())

		content := `{"base_item":{"id":"base"},"matrix":{"ENV":["v1","v2"]}}`
		tmpFile.WriteString(content)
		tmpFile.Close()

		var out bytes.Buffer
		stdout = &out
		exitCalled := false
		exitFunc = func(code int) {
			exitCalled = true
		}

		submitMatrixJob(server.URL, tmpFile.Name(), false)

		assert.False(t, exitCalled)
		assert.Contains(t, out.String(), "Matrix submission completed")
		assert.Contains(t, out.String(), "job-1, job-2")
	})

	t.Run("Failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		tmpFile, _ := os.CreateTemp("", "matrix-*.json")
		defer os.Remove(tmpFile.Name())
		tmpFile.WriteString(`{}`)
		tmpFile.Close()

		var out bytes.Buffer
		stdout = &out
		exitCalled := false
		exitFunc = func(code int) {
			exitCalled = true
		}

		submitMatrixJob(server.URL, tmpFile.Name(), false)

		assert.True(t, exitCalled)
		assert.Contains(t, out.String(), "Failed to submit matrix job")
	})

	t.Run("Partial Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"submitted": []string{"job-1"},
				"errors":    []string{"job-2: failed"},
			})
		}))
		defer server.Close()

		tmpFile, _ := os.CreateTemp("", "matrix-*.json")
		defer os.Remove(tmpFile.Name())
		tmpFile.WriteString(`{}`)
		tmpFile.Close()

		var out bytes.Buffer
		stdout = &out
		exitCalled := false
		exitFunc = func(code int) {
			exitCalled = true
		}

		submitMatrixJob(server.URL, tmpFile.Name(), false)

		assert.False(t, exitCalled)
		assert.Contains(t, out.String(), "job-1")
		assert.Contains(t, out.String(), "job-2: failed")
	})
}
