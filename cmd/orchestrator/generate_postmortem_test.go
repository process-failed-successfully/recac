package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGeneratePostmortem_SuccessStdout(t *testing.T) {
	var receivedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"postmortem": "# Postmortem Content"}`))
	}))
	defer server.Close()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	generatePostmortem(server.URL, "-", "my-tag", "my-match", "my-prov", "my-model")
	w.Close()
	out, _ := io.ReadAll(r)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, string(out), "# Postmortem Content")
	assert.Contains(t, receivedURL, "tag=my-tag")
	assert.Contains(t, receivedURL, "match=my-match")
	assert.Contains(t, receivedURL, "provider=my-prov")
	assert.Contains(t, receivedURL, "model=my-model")
}

func TestGeneratePostmortem_SuccessFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"postmortem": "# Postmortem Content"}`))
	}))
	defer server.Close()

	tmpFile, _ := os.CreateTemp("", "postmortem-*.md")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	generatePostmortem(server.URL, tmpFile.Name(), "", "", "", "")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, buf.String(), "Postmortem generated successfully and saved to")

	content, err := os.ReadFile(tmpFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, "# Postmortem Content", string(content))
}

func TestGeneratePostmortem_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`Internal error`))
	}))
	defer server.Close()

	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	generatePostmortem(server.URL, "-", "", "", "", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to generate postmortem: Internal error")
}

func TestGeneratePostmortem_ConnectionError(t *testing.T) {
	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	generatePostmortem("http://localhost:12345", "-", "", "", "", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, strings.ToLower(buf.String()), "failed to connect to orchestrator")
}

func TestGeneratePostmortem_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	oldStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	generatePostmortem(server.URL, "-", "", "", "", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to decode response")
}
