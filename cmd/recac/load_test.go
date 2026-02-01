package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// We need to use a mutex for the server request count because multiple workers might hit it
type threadSafeCounter struct {
	count int
	mu    sync.Mutex
}

func (c *threadSafeCounter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
}

func (c *threadSafeCounter) Get() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

func TestLoadCmd_Run(t *testing.T) {
	counter := &threadSafeCounter{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Inc()
		time.Sleep(1 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Reset flags
	loadUrl = server.URL
	loadRequests = 50
	loadConcurrency = 5
	loadDuration = 0
	loadMethod = "GET"
	loadBody = ""
	loadHeaders = nil

	err := runLoad(loadCmd, []string{})

	assert.NoError(t, err)
	assert.Equal(t, 50, counter.Get())
}

func TestLoadCmd_Duration(t *testing.T) {
	counter := &threadSafeCounter{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Inc()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	loadUrl = server.URL
	loadRequests = 0
	loadConcurrency = 2
	loadDuration = 100 * time.Millisecond
	loadMethod = "GET"
	loadBody = ""
	loadHeaders = nil

	err := runLoad(loadCmd, []string{})

	assert.NoError(t, err)
	assert.True(t, counter.Get() > 0, "Should have executed some requests")
}

func TestLoadCmd_HeadersAndBody(t *testing.T) {
	var capturedBody string
	var capturedHeader string
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		buf := make([]byte, 100)
		n, _ := r.Body.Read(buf)
		capturedBody = string(buf[:n])
		capturedHeader = r.Header.Get("X-Custom-Header")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	loadUrl = server.URL
	loadRequests = 1
	loadConcurrency = 1
	loadDuration = 0
	loadMethod = "POST"
	loadBody = "hello world"
	loadHeaders = []string{"X-Custom-Header: myvalue"}

	err := runLoad(loadCmd, []string{})

	assert.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "hello world", capturedBody)
	assert.Equal(t, "myvalue", capturedHeader)
}

func TestLoadCmd_Validation(t *testing.T) {
	loadUrl = ""
	err := runLoad(loadCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--url is required")
}
