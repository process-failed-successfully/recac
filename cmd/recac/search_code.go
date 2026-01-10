package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var searchCodeCmd = &cobra.Command{
	Use:   "search-code [pattern]",
	Short: "Search for a pattern in the workspace of all sessions",
	Long: `Scans through all files in each session's workspace and prints lines that match the provided pattern.
Each matching line is prefixed with the session name and file path for context.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := args[0]

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to initialize session manager: %w", err)
		}

		sessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}

		foundMatch := false
		ignoredDirs := map[string]bool{
			".git":         true,
			"node_modules": true,
			"vendor":       true,
			"dist":         true,
			"build":        true,
		}

		for _, session := range sessions {
			workspaceDir := filepath.Join(sm.SessionsDir(), session.Name, "workspace")
			if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
				continue
			}

			// DEBUG: Print the workspace directory being scanned
			// cmd.Printf("DEBUG: Scanning workspace: %s\n", workspaceDir)

			err := filepath.WalkDir(workspaceDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					if ignoredDirs[d.Name()] {
						return filepath.SkipDir
					}
					return nil
				}

				// Basic binary file check
				if strings.Contains(d.Name(), ".") && !strings.Contains(d.Name(), ".go") && !strings.Contains(d.Name(), ".txt") && !strings.Contains(d.Name(), ".md") && !strings.Contains(d.Name(), ".json") && !strings.Contains(d.Name(), ".yaml") && !strings.Contains(d.Name(), ".yml") && !strings.Contains(d.Name(), "Dockerfile") {
					return nil
				}

				file, err := os.Open(path)
				if err != nil {
					return nil // Skip files we can't open
				}

				scanner := bufio.NewScanner(file)
				lineNumber := 0
				for scanner.Scan() {
					lineNumber++
					line := scanner.Text()
					if strings.Contains(line, pattern) {
						relativePath, _ := filepath.Rel(workspaceDir, path)
						cmd.Printf("[%s:%s:%d] %s\n", session.Name, relativePath, lineNumber, line)
						foundMatch = true
					}
				}
				file.Close() // Explicitly close the file
				return nil
			})

			if err != nil {
				// Log the error but continue to the next session
				cmd.PrintErrf("error walking workspace for session %s: %v\n", session.Name, err)
			}
		}

		if !foundMatch {
			cmd.Println("No matches found.")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(searchCodeCmd)
}
