package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"recac/internal/runner"

	"github.com/spf13/cobra"
)

var archiveCmd = &cobra.Command{
	Use:   "archive [SESSION_NAME...]",
	Short: "Archive one or more sessions into a single zip file.",
	Long: `Archive packages the artifacts (logs, diffs, metadata) of one or more sessions
into a single zip file for sharing or long-term storage.

You can specify multiple session names as arguments or use the --all flag to archive all sessions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		all, _ := cmd.Flags().GetBool("all")
		output, _ := cmd.Flags().GetString("output")

		var sessionsToArchive []*runner.SessionState
		if all {
			sessions, err := sm.ListSessions()
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}
			sessionsToArchive = sessions
		} else {
			if len(args) == 0 {
				return fmt.Errorf("you must specify at least one session name or use the --all flag")
			}
			for _, sessionName := range args {
				session, err := sm.LoadSession(sessionName)
				if err != nil {
					return fmt.Errorf("failed to load session %s: %w", sessionName, err)
				}
				sessionsToArchive = append(sessionsToArchive, session)
			}
		}

		if len(sessionsToArchive) == 0 {
			cmd.Println("No sessions found to archive.")
			return nil
		}

		if output == "" {
			output = fmt.Sprintf("recac_archive_%s.zip", time.Now().Format("20060102150405"))
		}

		if err := createMultiSessionZip(output, sessionsToArchive); err != nil {
			return fmt.Errorf("failed to create archive: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully archived %d sessions to %s\n", len(sessionsToArchive), output)
		return nil
	},
}

func createMultiSessionZip(path string, sessions []*runner.SessionState) error {
	zipFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	gitClient := gitClientFactory()

	for _, session := range sessions {
		// Gather metadata
		metadata, err := json.MarshalIndent(session, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not marshal metadata for %s: %v\n", session.Name, err)
			continue
		}

		// Gather log file
		logContent, err := os.ReadFile(session.LogFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not read log for %s: %v\n", session.Name, err)
			// Continue, a session might not have a log file
		}

		// Gather git diff
		var diffContent []byte
		if session.StartCommitSHA != "" && session.EndCommitSHA != "" {
			diff, err := gitClient.Diff(session.Workspace, session.StartCommitSHA, session.EndCommitSHA)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not generate diff for %s: %v\n", session.Name, err)
			}
			diffContent = []byte(diff)
		}

		filesToArchive := map[string][]byte{
			"metadata.json": metadata,
			"session.log":   logContent,
			"work.diff":     diffContent,
		}

		for name, content := range filesToArchive {
			if len(content) == 0 {
				continue // Don't add empty files
			}
			// Use filepath.Join to create a subdirectory for each session
			filePathInZip := filepath.Join(session.Name, name)
			f, err := zipWriter.Create(filePathInZip)
			if err != nil {
				return fmt.Errorf("failed to create entry for %s in zip: %w", filePathInZip, err)
			}
			_, err = f.Write(content)
			if err != nil {
				return fmt.Errorf("failed to write content for %s to zip: %w", filePathInZip, err)
			}
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(archiveCmd)
	archiveCmd.Flags().Bool("all", false, "Archive all sessions.")
	archiveCmd.Flags().StringP("output", "o", "", "Output file path for the zip archive.")
}
