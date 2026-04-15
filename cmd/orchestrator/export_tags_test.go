package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestExportTags_JSON(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := TagStatsResponse{
			Tags: []TagPerformance{
				{Tag: "backend", TotalJobs: 5, SuccessfulJobs: 4, FailedJobs: 1, SuccessRate: 0.8},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	exportTags(server.URL, "-", "json", 10)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if !strings.Contains(buf.String(), `"tag": "backend"`) {
		t.Errorf("Expected 'backend' tag in output, got %s", buf.String())
	}
}

func TestExportTags_CSV(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := TagStatsResponse{
			Tags: []TagPerformance{
				{Tag: "frontend", TotalJobs: 10, SuccessfulJobs: 10, FailedJobs: 0, SuccessRate: 1.0},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	exportTags(server.URL, "-", "csv", 10)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if !strings.Contains(buf.String(), `frontend,10,10,0,1.00`) {
		t.Errorf("Expected 'frontend' csv row in output, got %s", buf.String())
	}
}
