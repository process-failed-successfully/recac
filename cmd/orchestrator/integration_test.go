package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestIntegration_SubmitAndRun(t *testing.T) {
	// Setup temp directory for file-dir poller
	tmpDir := t.TempDir()
	processedDir := filepath.Join(tmpDir, "processed")

	// Setup Viper configuration for this test
	viper.Reset()
	viper.Set("verbose", true)
	viper.Set("orchestrator.mode", "mock")
	viper.Set("orchestrator.poller", "file-dir")
	viper.Set("orchestrator.watch_dir", tmpDir)
	viper.Set("orchestrator.interval", "100ms") // Fast polling

	// Create context for orchestrator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run Orchestrator in background
	go func() {
		runOrchestratorWithContext(ctx, nil, nil)
	}()

	// Wait for startup (simple sleep)
	time.Sleep(500 * time.Millisecond)

	// Submit a task using the submit logic directly (or via command)
	// We'll call submitCmd.RunE to test the command logic too
	// But we need to ensure flags are parsed or bound.
	// Since we use viper.GetString in submitCmd, and we set viper above, it should work.
	// But submitCmd also reads flags. We need to mock flags or just call the logic helper if easier.
	// Actually, submitCmd uses:
	// summary, _ := cmd.Flags().GetString("summary")
	// So we need to set flags on the command object we pass.

	// Let's call the helper submitToFileDir to avoid Cobra flag complexity in test
	// But wait, we want to test "submit" command integration.
	// We can set flags on submitCmd.
	submitCmd.Flags().Set("summary", "Integration Task")
	submitCmd.Flags().Set("id", "INT-1")

	err := submitCmd.RunE(submitCmd, nil)
	require.NoError(t, err)

	// Wait for processing
	// The orchestrator polls every 100ms.
	// It should pick up the file and move it to "processed".

	deadline := time.Now().Add(5 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(processedDir, "INT-1.json")); err == nil {
			found = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	require.True(t, found, "Task file should have been moved to processed directory")
}
