package web

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestServer_StartStop(t *testing.T) {
	mockStore := new(TestifyMockStore)
	server := NewServer(mockStore, 0, "test-project") // 0 for random port

	// Channel to signal we are about to call Start
	started := make(chan struct{})

	go func() {
		close(started)
		// Start blocks until Stop is called or error
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server failed: %v\n", err)
		}
	}()

	<-started
	// Wait a bit for ListenAndServe to actually bind and start loop
	time.Sleep(100 * time.Millisecond)

	// Stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Stop(ctx)
	assert.NoError(t, err)
}

func TestServer_Stop_NotStarted(t *testing.T) {
	mockStore := new(TestifyMockStore)
	server := NewServer(mockStore, 8080, "test-project")

	err := server.Stop(context.Background())
	assert.NoError(t, err)
}
