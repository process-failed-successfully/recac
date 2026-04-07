package main

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
)

func TestAllowedPollers(t *testing.T) {
	// Setup generic context & logger
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name          string
		poller        string
		allowedList   []string
		expectedError string
	}{
		{
			name:        "Poller in allowed list succeeds",
			poller:      "file",
			allowedList: []string{"file", "github"},
		},
		{
			name:        "Empty poller allowed if jira is in list",
			poller:      "", // defaults to jira
			allowedList: []string{"jira", "github"},
		},
		{
			name:          "Poller not in allowed list fails",
			poller:        "gitlab",
			allowedList:   []string{"jira", "file"},
			expectedError: "poller 'gitlab' is not in the allowed pollers list: [jira file]",
		},
		{
			name:          "Empty poller fails if jira not in list",
			poller:        "",
			allowedList:   []string{"file", "github"},
			expectedError: "poller 'jira' is not in the allowed pollers list: [file github]",
		},
		{
			name:        "Empty allowed list means all pollers allowed",
			poller:      "file",
			allowedList: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("orchestrator.poller", tt.poller)
			viper.Set("orchestrator.allowed_pollers", tt.allowedList)

			// Provide required configs so we don't fail for other reasons
			// For "file" poller
			viper.Set("orchestrator.work_file", "dummy.json")
			// For "github" poller
			viper.Set("orchestrator.github_token", "dummy")
			viper.Set("orchestrator.github_owner", "dummy")
			viper.Set("orchestrator.github_repo", "dummy")
			// Make sure we just do a quick exit via verify if it gets that far
			viper.Set("orchestrator.verify", true)
			viper.Set("orchestrator.db_file", ":memory:")
			viper.Set("orchestrator.mode", "process")

			// Some tests are failing with 'Post "/scale": unsupported protocol scheme ""'.
			// Set orchestrator.scale to its default value -1 so we bypass the scale branch.
			viper.Set("orchestrator.scale", -1)

			// Some tests will fail because we are providing bogus connection info,
			// or simply that "jira" / "gitlab" connections fail when pinging.
			// The key is that the "not allowed" error must match exactly if expected.

			err := run(ctx, logger)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				// If no error is expected from the poller check, it might still fail
				// from something else (like failing to init jira client because there's no real URL).
				// We just need to make sure it did NOT fail because of the allowed pollers list.
				if err != nil {
					assert.NotContains(t, err.Error(), "is not in the allowed pollers list")
				}
			}
		})
	}
}
