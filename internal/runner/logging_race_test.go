package runner

import (
	"context"
	"recac/internal/docker"
	"sync"
	"testing"
)

// MockAgentForRace is a minimal agent for NewSession
type MockAgentForRace struct{}

func (m *MockAgentForRace) Send(ctx context.Context, prompt string) (string, error) {
	return "", nil
}

func (m *MockAgentForRace) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return "", nil
}

func TestInitializeLoggingRace(t *testing.T) {
	// Set log directory globally for this test to avoid race on t.Setenv
	tmpDir := t.TempDir()
	t.Setenv("RECAC_LOGS_DIR", tmpDir)

	// Simulate parallel execution
	var wg sync.WaitGroup
	// Create enough concurrency to trigger potential race conditions
	concurrency := 20

	mockDocker, _ := docker.NewMockClient()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Mock agent
			mockAgent := &MockAgentForRace{}

			// NewSession initializes DB and Logging.
			// It might print to stderr/stdout.
			// We mainly want to see if it PANICS or errors out badly.
            _ = NewSession(mockDocker, mockAgent, tmpDir, "image", "project", "provider", "model", 1)
		}(i)
	}
	wg.Wait()
}
