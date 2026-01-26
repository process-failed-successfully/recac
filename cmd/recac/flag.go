package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagKeep   bool
	flagDryRun bool
)

var flagCmd = &cobra.Command{
	Use:   "flag",
	Short: "Manage feature flags",
	Long:  `Manage feature flags lifecycle: list, add, and cleanup (using AI).`,
}

var flagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all feature flags used in code",
	RunE:  runFlagList,
}

var flagAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Add a new feature flag",
	Args:  cobra.ExactArgs(1),
	RunE:  runFlagAdd,
}

var flagCleanupCmd = &cobra.Command{
	Use:   "cleanup [name]",
	Short: "Cleanup a feature flag using AI",
	Long:  `Finds all usages of a feature flag and uses AI to refactor the code, permanently keeping the enabled or disabled state.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runFlagCleanup,
}

func init() {
	rootCmd.AddCommand(flagCmd)
	flagCmd.AddCommand(flagListCmd)
	flagCmd.AddCommand(flagAddCmd)
	flagCmd.AddCommand(flagCleanupCmd)

	flagCleanupCmd.Flags().BoolVar(&flagKeep, "keep", true, "Keep the 'true' branch (enabled) or 'false' branch (disabled)")
	flagCleanupCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Preview changes without modifying files")
}

func runFlagList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	flags := make(map[string]bool)
	// Regex to find viper.GetBool("features.NAME")
	// We allow quotes around the string key
	re := regexp.MustCompile(`viper\.GetBool\("features\.([a-zA-Z0-9_]+)"\)`)

	ignoreMap := DefaultIgnoreMap()

	err = filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			if ignoreMap[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		if filepath.Ext(path) != ".go" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip unreadable
		}

		matches := re.FindAllStringSubmatch(string(content), -1)
		for _, match := range matches {
			if len(match) > 1 {
				flags[match[1]] = true
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	if len(flags) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No feature flags found (looking for 'features.NAME' pattern).")
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Found feature flags:")
	for f := range flags {
		fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", f)
	}

	return nil
}

func runFlagAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	key := fmt.Sprintf("features.%s", name)

	// Check if already exists
	if viper.IsSet(key) {
		return fmt.Errorf("feature flag '%s' already exists", name)
	}

	// Set value
	viper.Set(key, false)

	// Save
	if err := viper.WriteConfig(); err != nil {
		// If no config file found, try creating one?
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Try writing to default
			if err := viper.SafeWriteConfigAs("config.yaml"); err != nil {
				return fmt.Errorf("failed to create config file: %w", err)
			}
		} else {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Added feature flag '%s' to config\n", name)
	fmt.Fprintln(cmd.OutOrStdout(), "\nUsage in Go:")
	fmt.Fprintf(cmd.OutOrStdout(), "if viper.GetBool(\"%s\") {\n    // New feature code\n}\n", key)

	return nil
}

func runFlagCleanup(cmd *cobra.Command, args []string) error {
	flagName := args[0]
	fullFlagName := fmt.Sprintf("features.%s", flagName)
	targetState := "ENABLED"
	if !flagKeep {
		targetState = "DISABLED"
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// 1. Find files
	var affectedFiles []string
	re := regexp.MustCompile(regexp.QuoteMeta(fullFlagName))
	ignoreMap := DefaultIgnoreMap()

	filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() {
				if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
					return filepath.SkipDir
				}
				if ignoreMap[info.Name()] {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err == nil {
			if re.Match(content) {
				affectedFiles = append(affectedFiles, path)
			}
		}
		return nil
	})

	if len(affectedFiles) == 0 {
		return fmt.Errorf("no usages found for flag '%s'", flagName)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d files using flag '%s'. Starting cleanup (%s)...\n", len(affectedFiles), flagName, targetState)

	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-flag-cleanup")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	for _, file := range affectedFiles {
		fmt.Fprintf(cmd.OutOrStdout(), "Processing %s...\n", file)
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to read %s: %v\n", file, err)
			continue
		}

		prompt := fmt.Sprintf(`Refactor the following Go code to remove a feature flag.
Flag: "%s"
Desired State: %s (Keep the code path as if the flag is %v)

Remove the "if" check for the flag.
If State is ENABLED, keep the code inside the "if" block (and remove the "else" block).
If State is DISABLED, remove the "if" block and keep the "else" block (if any).
Ensure imports are updated if necessary.
Return ONLY the full content of the file.

Code:
%s`, fullFlagName, targetState, flagKeep, string(content))

		resp, err := ag.Send(ctx, prompt)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Agent failed for %s: %v\n", file, err)
			continue
		}

		cleaned := utils.CleanCodeBlock(resp)

		if flagDryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would update %s:\n%s\n", file, cleaned)
		} else {
			if err := os.WriteFile(file, []byte(cleaned), 0644); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to write %s: %v\n", file, err)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Updated %s\n", file)
			}
		}
	}

	if flagDryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "Dry run complete. No files changed.")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Cleanup complete. Please run tests to verify.")
	}
	return nil
}
