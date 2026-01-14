package ui

import (
	"bytes"
	"recac/internal/model"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

// TestStartPsDashboard is a simple test to ensure the dashboard can be started
// without panicking. It uses a custom IO reader to immediately quit the program.
func TestStartPsDashboard(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.Bool("costs", false, "")

	// This is a bit of a hack to prevent the TUI from actually running
	// and blocking the test. We send a "q" to the input to quit immediately.
	var in bytes.Buffer
	in.WriteString("q\n")

	// Mock the session fetcher
	mockFetcher := func(flags *pflag.FlagSet) ([]model.UnifiedSession, []string, error) {
		return []model.UnifiedSession{
			{Name: "session-1", Status: "running", Location: "local", StartTime: time.Now()},
		}, nil, nil
	}

	// Run the program and check for errors
	err := StartPsDashboard(flags, mockFetcher)
	assert.NoError(t, err)
}
