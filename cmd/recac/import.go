package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"recac/internal/runner"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import [ZIP_FILE]",
	Short: "Import a session from a zip archive",
	Long: `Import a previously exported session from a zip archive.
This restores the session metadata and logs, allowing you to view or replay the session.
If a session with the same name already exists, the imported session will be renamed.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		zipPath := args[0]

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		// Create a temp directory for extraction
		tempDir, err := os.MkdirTemp("", "recac-import-*")
		if err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}
		defer os.RemoveAll(tempDir)

		if err := unzip(zipPath, tempDir); err != nil {
			return fmt.Errorf("failed to unzip archive: %w", err)
		}

		// Read metadata
		metadataPath := filepath.Join(tempDir, "metadata.json")
		metadataBytes, err := os.ReadFile(metadataPath)
		if err != nil {
			return fmt.Errorf("failed to read metadata.json: %w", err)
		}

		var session runner.SessionState
		if err := json.Unmarshal(metadataBytes, &session); err != nil {
			return fmt.Errorf("failed to parse metadata.json: %w", err)
		}

		// Check for name conflict
		originalName := session.Name
		newName := originalName

		// If session exists, append timestamp
		if _, err := sm.LoadSession(newName); err == nil {
			newName = fmt.Sprintf("%s-imported-%d", originalName, time.Now().Unix())
			fmt.Fprintf(cmd.OutOrStdout(), "Session '%s' already exists. Renaming import to '%s'\n", originalName, newName)
			session.Name = newName
		}

		// Copy log file
		logPath := filepath.Join(tempDir, "session.log")
		if _, err := os.Stat(logPath); err == nil {
			destLogPath := filepath.Join(sm.SessionsDir(), newName+".log")
			if err := copyFile(logPath, destLogPath); err != nil {
				return fmt.Errorf("failed to copy log file: %w", err)
			}
			session.LogFile = destLogPath
		} else {
			// If no log file, we should probably warn or set to empty
			fmt.Fprintln(cmd.ErrOrStderr(), "Warning: session.log not found in archive")
			session.LogFile = ""
		}

		// Copy work.diff if exists (optional)
		diffPath := filepath.Join(tempDir, "work.diff")
		if _, err := os.Stat(diffPath); err == nil {
			destDiffPath := filepath.Join(sm.SessionsDir(), newName+".diff")
			if err := copyFile(diffPath, destDiffPath); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to copy work.diff: %v\n", err)
			}
		}

		// Clear AgentStateFile as it's likely invalid
		session.AgentStateFile = ""

		// Save session
		if err := sm.SaveSession(&session); err != nil {
			return fmt.Errorf("failed to save imported session: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully imported session '%s'\n", newName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}
