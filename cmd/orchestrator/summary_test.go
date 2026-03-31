package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSummaryJobs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/summary", func(w http.ResponseWriter, r *http.Request) {
		summary := map[string]int{
			"Completed": 5,
			"Failed":    2,
			"Running":   1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summary)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	t.Run("Text Format", func(t *testing.T) {
		var out bytes.Buffer
		oldStdout := stdout
		stdout = &out
		defer func() { stdout = oldStdout }()

		summaryJobs(server.URL, "text")

		output := out.String()
		assert.Contains(t, output, "Job Summary (8 total)")
		assert.Contains(t, output, "Completed:")
		assert.Contains(t, output, "5")
		assert.Contains(t, output, "Failed:")
		assert.Contains(t, output, "2")
		assert.Contains(t, output, "Running:")
		assert.Contains(t, output, "1")
	})

	t.Run("JSON Format", func(t *testing.T) {
		var out bytes.Buffer
		oldStdout := stdout
		stdout = &out
		defer func() { stdout = oldStdout }()

		summaryJobs(server.URL, "json")

		var result map[string]interface{}
		err := json.Unmarshal(out.Bytes(), &result)
		assert.NoError(t, err)

		assert.Equal(t, float64(8), result["total"])

		summaryMap, ok := result["summary"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, float64(5), summaryMap["Completed"])
		assert.Equal(t, float64(2), summaryMap["Failed"])
		assert.Equal(t, float64(1), summaryMap["Running"])
	})
}
