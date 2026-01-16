package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"recac/internal/runner"

	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import <ZIP_FILE> [NEW_NAME]",
	Short: "Import a session from a zip archive",
	Long: `Import a session from a zip archive (previously exported using 'recac export').
This restores the session metadata, logs, and git diffs for inspection.
The session status will be set to 'stopped' or 'completed'.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		zipPath := args[0]
		var newName string
		if len(args) > 1 {
			newName = args[1]
		}

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		// Verify zip file exists
		if _, err := os.Stat(zipPath); err != nil {
			return fmt.Errorf("failed to stat zip file: %w", err)
		}

		// Open zip archive
		r, err := zip.OpenReader(zipPath)
		if err != nil {
			return fmt.Errorf("failed to open zip file: %w", err)
		}
		defer r.Close()

		// Read metadata.json
		var sessionState runner.SessionState
		var metadataFound bool
		var logFile *zip.File
		var diffFile *zip.File

		for _, f := range r.File {
			switch f.Name {
			case "metadata.json":
				rc, err := f.Open()
				if err != nil {
					return fmt.Errorf("failed to open metadata.json in zip: %w", err)
				}
				data, err := io.ReadAll(rc)
				rc.Close()
				if err != nil {
					return fmt.Errorf("failed to read metadata.json: %w", err)
				}
				if err := json.Unmarshal(data, &sessionState); err != nil {
					return fmt.Errorf("failed to parse metadata.json: %w", err)
				}
				metadataFound = true
			case "session.log":
				logFile = f
			case "work.diff":
				diffFile = f
			}
		}

		if !metadataFound {
			return fmt.Errorf("invalid archive: metadata.json not found")
		}

		// Determine target session name
		targetName := sessionState.Name
		if newName != "" {
			targetName = newName
		}

		// Sanitize session name to prevent path traversal
		if filepath.Base(targetName) != targetName {
			return fmt.Errorf("invalid session name '%s': path traversal characters detected", targetName)
		}

		// Check for collision
		if _, err := sm.LoadSession(targetName); err == nil {
			// Session exists
			return fmt.Errorf("session '%s' already exists", targetName)
		}

		// Prepare destination paths
		sessionsDir := sm.SessionsDir()
		targetLogPath := filepath.Join(sessionsDir, targetName+".log")
		targetDiffPath := filepath.Join(sessionsDir, targetName+".diff")

		// Update SessionState
		sessionState.Name = targetName
		sessionState.LogFile = targetLogPath
		sessionState.PID = 0
		if sessionState.Status == "running" || sessionState.Status == "paused" {
			sessionState.Status = "stopped" // Force stopped state for imported sessions
		}
		// Note: We keep sessionState.Workspace as is, even if it doesn't exist on this machine,
		// to preserve historical record.

		// Extract log file
		if logFile != nil {
			rc, err := logFile.Open()
			if err != nil {
				return fmt.Errorf("failed to open session.log in zip: %w", err)
			}
			outFile, err := os.Create(targetLogPath)
			if err != nil {
				rc.Close()
				return fmt.Errorf("failed to create log file %s: %w", targetLogPath, err)
			}
			_, err = io.Copy(outFile, rc)
			outFile.Close()
			rc.Close()
			if err != nil {
				return fmt.Errorf("failed to write log file: %w", err)
			}
		} else {
			// Create empty log file if missing
			os.Create(targetLogPath)
		}

		// Extract diff file (optional)
		if diffFile != nil {
			rc, err := diffFile.Open()
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to open work.diff in zip: %v\n", err)
			} else {
				outFile, err := os.Create(targetDiffPath)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to create diff file %s: %v\n", targetDiffPath, err)
				} else {
					_, err = io.Copy(outFile, rc)
					outFile.Close()
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to write diff file: %v\n", err)
					}
				}
				rc.Close()
			}
		}

		// Save new session state
		if err := sm.SaveSession(&sessionState); err != nil {
			// Cleanup on failure
			os.Remove(targetLogPath)
			os.Remove(targetDiffPath)
			return fmt.Errorf("failed to save imported session state: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully imported session '%s'\n", targetName)
		if _, err := os.Stat(sessionState.Workspace); os.IsNotExist(err) {
			fmt.Fprintf(cmd.OutOrStdout(), "Note: Original workspace '%s' does not exist on this machine.\n", sessionState.Workspace)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
}
