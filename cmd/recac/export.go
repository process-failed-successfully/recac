package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export [SESSION_NAME]",
	Short: "Export a session's artifacts to a zip file.",
	Long: `Export a session's artifacts (logs, diffs, metadata) to a zip file
for sharing or archival.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]
		output, _ := cmd.Flags().GetString("output")
		if output == "" {
			output = fmt.Sprintf("%s.zip", sessionName)
		}

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}
		session, err := sm.LoadSession(sessionName)
		if err != nil {
			return fmt.Errorf("failed to get session %s: %w", sessionName, err)
		}

		// Gather metadata
		metadata, err := json.MarshalIndent(session, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal session metadata: %w", err)
		}

		// Gather log file
		logContent, err := os.ReadFile(session.LogFile)
		if err != nil {
			return fmt.Errorf("failed to read log file: %w", err)
		}

		// Gather git diff
		var diffContent []byte
		if session.StartCommitSHA != "" && session.EndCommitSHA != "" {
			// Use the same factory pattern as workdiff for testability
			gitClient := gitNewClient()
			diff, err := gitClient.Diff(session.Workspace, session.StartCommitSHA, session.EndCommitSHA)
			if err != nil {
				// Not a fatal error, just a warning.
				fmt.Fprintf(cmd.ErrOrStderr(), "Could not generate diff: %v\n", err)
			}
			diffContent = []byte(diff)
		}

		// Create the zip archive.
		if err := createZipArchive(output, metadata, logContent, diffContent); err != nil {
			return fmt.Errorf("failed to create zip archive: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully exported session %s to %s\n", sessionName, output)
		return nil
	},
}

func createZipArchive(path string, metadata, logContent, diffContent []byte) error {
	zipFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	filesToArchive := map[string][]byte{
		"metadata.json": metadata,
		"session.log":   logContent,
		"work.diff":     diffContent,
	}

	for name, content := range filesToArchive {
		if len(content) == 0 {
			continue // Don't add empty files to the archive
		}
		f, err := zipWriter.Create(name)
		if err != nil {
			return err
		}
		_, err = f.Write(content)
		if err != nil {
			return err
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().StringP("output", "o", "", "Output file path for the zip archive.")
}
