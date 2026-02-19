package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockHTTPClient struct {
	PostFunc func(url, contentType string, body io.Reader) (*http.Response, error)
}

func (m *MockHTTPClient) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	if m.PostFunc != nil {
		return m.PostFunc(url, contentType, body)
	}
	return nil, errors.New("not implemented")
}

func TestSubmitJob(t *testing.T) {
	// Create a temp file
	tmpfile, err := os.CreateTemp("", "job.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err := tmpfile.Write([]byte(`{"id":"job1"}`)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	mockClient := &MockHTTPClient{
		PostFunc: func(url, contentType string, body io.Reader) (*http.Response, error) {
			assert.Contains(t, url, "/jobs")
			assert.Equal(t, "application/json", contentType)
			return &http.Response{
				StatusCode: http.StatusAccepted,
				Body:       io.NopCloser(bytes.NewBufferString("Job submitted")),
			}, nil
		},
	}

	var output bytes.Buffer
	exitCalled := false
	exitFunc := func(code int) {
		exitCalled = true
	}

	submitJob("http://localhost", tmpfile.Name(), mockClient, &output, exitFunc)

	assert.False(t, exitCalled)
	assert.Contains(t, output.String(), "Job submitted")
}

func TestSubmitJob_InvalidFile(t *testing.T) {
	var output bytes.Buffer
	exitCode := 0
	exitFunc := func(code int) {
		exitCode = code
	}

	submitJob("http://localhost", "non-existent.json", nil, &output, exitFunc)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, output.String(), "Failed to open file")
}

func TestSubmitJob_InvalidJSON(t *testing.T) {
	tmpfile, _ := os.CreateTemp("", "job.json")
	defer os.Remove(tmpfile.Name())
	tmpfile.Write([]byte(`invalid-json`))
	tmpfile.Close()

	var output bytes.Buffer
	exitCode := 0
	exitFunc := func(code int) {
		exitCode = code
	}

	submitJob("http://localhost", tmpfile.Name(), nil, &output, exitFunc)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, output.String(), "Invalid JSON")
}

func TestSubmitAdHocJob(t *testing.T) {
	mockClient := &MockHTTPClient{
		PostFunc: func(url, contentType string, body io.Reader) (*http.Response, error) {
			assert.Contains(t, url, "/jobs")
			assert.Equal(t, "application/json", contentType)

			bodyBytes, err := io.ReadAll(body)
			if err != nil {
				return nil, err
			}
			bodyStr := string(bodyBytes)
			assert.Contains(t, bodyStr, `"Summary":"task"`)
			assert.Contains(t, bodyStr, `"Description":"task"`)
			assert.Contains(t, bodyStr, `"RepoURL":"repo"`)
			assert.Contains(t, bodyStr, `"ID":"id"`)

			return &http.Response{
				StatusCode: http.StatusAccepted,
				Body:       io.NopCloser(bytes.NewBufferString("Ad-hoc job submitted")),
			}, nil
		},
	}

	var output bytes.Buffer
	exitCalled := false
	exitFunc := func(code int) {
		exitCalled = true
	}

	submitAdHocJob("http://localhost", "repo", "task", "id", mockClient, &output, exitFunc)

	assert.False(t, exitCalled)
	assert.Contains(t, output.String(), "Ad-hoc job submitted")
}

func TestSubmitAdHocJob_Error(t *testing.T) {
	mockClient := &MockHTTPClient{
		PostFunc: func(url, contentType string, body io.Reader) (*http.Response, error) {
			return nil, errors.New("network error")
		},
	}

	var output bytes.Buffer
	exitCode := 0
	exitFunc := func(code int) {
		exitCode = code
	}

	submitAdHocJob("http://localhost", "repo", "task", "id", mockClient, &output, exitFunc)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, output.String(), "Failed to connect")
}
