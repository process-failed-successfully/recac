package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExportTagsCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tags": [{"tag": "frontend", "total_jobs": 10, "successful_jobs": 8, "failed_jobs": 2, "success_rate": 0.8}]}`))
	}))
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitFunc = func(code int) {}
	defer func() { exitFunc = oldExit }()

	exportTags(server.URL, "-", "json", 10)

	output := out.String()

	if !strings.Contains(output, `"tag": "frontend"`) {
		t.Errorf("Expected frontend tag in output, got: %s", output)
	}
}
