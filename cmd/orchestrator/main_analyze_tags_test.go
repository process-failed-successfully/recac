package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestExportTagsCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tags": [{"tag": "frontend", "total_jobs": 10, "successful_jobs": 8, "failed_jobs": 2, "success_rate": 0.8}]}`))
	}))
	defer server.Close()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exportTags(server.URL, "-", "json", 10)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, `"tag": "frontend"`) {
		t.Errorf("Expected frontend tag in output, got: %s", output)
	}
}
