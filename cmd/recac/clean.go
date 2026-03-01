package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
)

var cleanDryRun bool

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "Preview which files would be deleted without actually deleting them")
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove temporary files created during the session and repository debris",
	Long: `Cleans up ephemeral files to maintain a lean repository state.

It specifically targets:
- Output files (coverage.out)
- Common agent outputs (app_spec.txt, feature_list.json)
- RECAC internal temporary files (.recac-*.log)
- Any files listed in temp_files.txt created by legacy recac commands.`,
	RunE: runCleanCmd,
}

func runCleanCmd(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🧹 Cleaning up temporary files and repository debris...")
	if cleanDryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "   [DRY RUN MODE - no files will be actually deleted]")
	}

	deletedCount := 0

	// 1. Process legacy temp_files.txt
	tempFilesPath := filepath.Join(cwd, "temp_files.txt")
	lines, err := utils.ReadLines(tempFilesPath)
	if err == nil {
		var filesToRemove []string
		for _, line := range lines {
			if line != "" {
				filesToRemove = append(filesToRemove, line)
			}
		}

		for _, f := range filesToRemove {
			absPath, err := filepath.Abs(f)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error resolving path %s: %v\n", f, err)
				continue
			}

			if err := deleteFile(cmd, absPath); err == nil {
				deletedCount++
			}
		}

		if cleanDryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "Would remove %s\n", tempFilesPath)
		} else {
			os.Remove(tempFilesPath)
		}
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error opening %s: %v\n", tempFilesPath, err)
	}

	// 2. Scan for specific patterns
	// Note: We use WalkDir instead of Walk to avoid redundant lstat calls
	ignoreMap := DefaultIgnoreMap()

	err = filepath.WalkDir(cwd, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors accessing specific paths
		}

		// Skip ignored directories to avoid over-cleaning (like .git, node_modules)
		if d.IsDir() {
			name := d.Name()
			if ignoreMap[name] || (strings.HasPrefix(name, ".") && name != "." && name != "..") {
				return filepath.SkipDir
			}
			return nil
		}

		name := d.Name()
		shouldDelete := false

		// Known ephemeral exact filenames
		if name == "coverage.out" || name == "app_spec.txt" || name == "feature_list.json" {
			shouldDelete = true
		}

		// Common recac temp logs (but not standard app logs)
		if strings.HasPrefix(name, ".recac-") && strings.HasSuffix(name, ".log") {
			shouldDelete = true
		}

		if shouldDelete {
			if err := deleteFile(cmd, path); err == nil {
				deletedCount++
			}
		}

		return nil
	})

	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error walking directory: %v\n", err)
	}

	if deletedCount == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✨ Repository is already clean. No files removed.")
	} else {
		if cleanDryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "✅ Dry run complete. %d files would be removed.\n", deletedCount)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "✅ Cleanup complete. Removed %d files.\n", deletedCount)
		}
	}

	return nil
}

func deleteFile(cmd *cobra.Command, path string) error {
	// Verify it still exists before deleting
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return err
	}

	if cleanDryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Would remove %s\n", path)
		return nil
	}

	err := os.Remove(path)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error removing %s: %v\n", path, err)
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Removed %s\n", path)
	return nil
}
