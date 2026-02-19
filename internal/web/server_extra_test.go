package web

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewServer_DefaultProject(t *testing.T) {
	server := NewServer(nil, 8080, "")
	assert.Equal(t, "default", server.projectID)
}

func TestServer_Start_Stop(t *testing.T) {
	// Use port 0 for random port
	server := NewServer(nil, 0, "test")

	errChan := make(chan error)
	go func() {
		errChan <- server.Start()
	}()

	// Wait for server to start (naive sleep)
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := server.Stop(ctx)
	assert.NoError(t, err)

	// Server should return http.ErrServerClosed
	select {
	case err := <-errChan:
		assert.Equal(t, http.ErrServerClosed, err)
	case <-time.After(2 * time.Second):
		t.Error("Server did not stop")
	}
}
