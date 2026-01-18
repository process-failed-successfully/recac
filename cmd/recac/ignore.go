package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	ignoreDocker bool
	ignoreRemove bool
	ignoreList   bool
)

var ignoreCmd = &cobra.Command{
	Use:   "ignore [pattern]",
	Short: "Add or remove patterns from .gitignore or .dockerignore",
	Long:  `Manage ignored files and directories. By default, it operates on .gitignore. Use --docker to operate on .dockerignore.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fileName := ".gitignore"
		if ignoreDocker {
			fileName = ".dockerignore"
		}

		// Handle List (no args or --list)
		if len(args) == 0 || ignoreList {
			return listPatterns(fileName, cmd)
		}

		pattern := args[0]
		if ignoreRemove {
			return removePattern(fileName, pattern, cmd)
		}
		return addPattern(fileName, pattern, cmd)
	},
}

func init() {
	rootCmd.AddCommand(ignoreCmd)
	ignoreCmd.Flags().BoolVar(&ignoreDocker, "docker", false, "Use .dockerignore instead of .gitignore")
	ignoreCmd.Flags().BoolVarP(&ignoreRemove, "remove", "r", false, "Remove the pattern")
	ignoreCmd.Flags().BoolVarP(&ignoreList, "list", "l", false, "List ignored patterns")
}

func listPatterns(filename string, cmd *cobra.Command) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(cmd.OutOrStdout(), "%s does not exist.\n", filename)
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", filename, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Contents of %s:\n", filename)
	fmt.Fprintln(cmd.OutOrStdout(), string(content))
	return nil
}

func addPattern(filename, pattern string, cmd *cobra.Command) error {
	contentBytes, err := os.ReadFile(filename)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", filename, err)
	}
	content := string(contentBytes)

	// Check if pattern already exists
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == pattern {
			fmt.Fprintf(cmd.OutOrStdout(), "Pattern '%s' already exists in %s\n", pattern, filename)
			return nil
		}
	}

	// Prepare to append
	toWrite := pattern + "\n"
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		toWrite = "\n" + toWrite
	}

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", filename, err)
	}
	defer f.Close()

	if _, err := f.WriteString(toWrite); err != nil {
		return fmt.Errorf("failed to write to %s: %w", filename, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Added '%s' to %s\n", pattern, filename)
	return nil
}

func removePattern(filename, pattern string, cmd *cobra.Command) error {
	contentBytes, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s does not exist", filename)
		}
		return err
	}

	lines := strings.Split(string(contentBytes), "\n")
	var newLines []string
	found := false
	for _, line := range lines {
		if strings.TrimSpace(line) == pattern {
			found = true
			continue
		}
		newLines = append(newLines, line)
	}

	if !found {
		fmt.Fprintf(cmd.OutOrStdout(), "Pattern '%s' not found in %s\n", pattern, filename)
		return nil
	}

	// Reconstruct the file content
	// If the last line was empty (resulting from split), newLines might contain an empty string at the end.
	// If we Join with \n, we preserve that structure mostly.
	// However, if we removed the last line, we want to ensure we don't leave double newlines or no newline at end?
	// Simply joining with \n is usually sufficient for gitignore.

	// Edge case: if the file ends with \n, Split gives "" as the last element.
	// If we remove an item, we just reconstruct.

	output := strings.Join(newLines, "\n")

	if err := os.WriteFile(filename, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to update %s: %w", filename, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Removed '%s' from %s\n", pattern, filename)
	return nil
}
