package main

import (
	"context"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var flagCmd = &cobra.Command{
	Use:   "flag",
	Short: "Manage feature flags",
	Long:  `Manage feature flags in the project configuration. allows listing, adding, and cleaning up flags.`,
}

var flagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all feature flags",
	Run: func(cmd *cobra.Command, args []string) {
		flags := viper.GetStringMap("feature_flags")
		if len(flags) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No feature flags found.")
			return
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %s\n", "NAME", "STATUS", "DESCRIPTION")
		fmt.Fprintln(cmd.OutOrStdout(), "------------------------------------------------------------")

		// Sort keys for consistent output
		var keys []string
		for k := range flags {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, name := range keys {
			val := flags[name]
			details, ok := val.(map[string]interface{})
			status := "disabled"
			desc := ""

			if ok {
				if v, ok := details["enabled"].(bool); ok && v {
					status = "enabled"
				}
				if d, ok := details["description"].(string); ok {
					desc = d
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %s\n", name, status, desc)
		}
	},
}

var (
	flagDesc   string
	flagEnable bool
)

var flagAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Add a new feature flag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		flags := viper.GetStringMap("feature_flags")

		if _, exists := flags[name]; exists {
			return fmt.Errorf("feature flag '%s' already exists", name)
		}

		newFlag := map[string]interface{}{
			"enabled":     flagEnable,
			"description": flagDesc,
		}

		// Viper doesn't support setting nested keys in a map easily if the map is retrieved as interface{}
		// So we construct the map path
		viper.Set("feature_flags."+name, newFlag)

		if err := viper.WriteConfig(); err != nil {
			// If no config file exists, try to create one or safe write
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				return viper.SafeWriteConfig()
			}
			return fmt.Errorf("failed to write config: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "✅ Feature flag '%s' added (enabled=%v)\n", name, flagEnable)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(flagCmd)
	flagCmd.AddCommand(flagListCmd)
	flagCmd.AddCommand(flagAddCmd)

	flagAddCmd.Flags().StringVar(&flagDesc, "desc", "", "Description of the feature flag")
	flagAddCmd.Flags().BoolVar(&flagEnable, "enable", false, "Enable the feature flag by default")
	flagCmd.AddCommand(flagCleanupCmd)
}

var flagCleanupCmd = &cobra.Command{
	Use:   "cleanup [name]",
	Short: "Refactor code to remove a feature flag using AI",
	Long:  `Searches for usages of the feature flag and asks AI to refactor the code to remove the flag check, effectively graduating the feature.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		flagName := args[0]
		flags := viper.GetStringMap("feature_flags")
		if _, exists := flags[flagName]; !exists {
			return fmt.Errorf("feature flag '%s' not found in config", flagName)
		}

		// 1. Find files
		fmt.Fprintf(cmd.OutOrStdout(), "🔍 Searching for usages of '%s'...\n", flagName)
		files, err := findFilesWithContent(flagName)
		if err != nil {
			return fmt.Errorf("failed to search files: %w", err)
		}

		if len(files) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No usages found in code. Removing from config only.")
			return removeFlagFromConfig(cmd, flagName)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Found usages in %d files:\n", len(files))
		for _, f := range files {
			fmt.Fprintf(cmd.OutOrStdout(), " - %s\n", f)
		}

		// 2. Initialize AI
		ctx := context.Background()
		provider := viper.GetString("provider")
		model := viper.GetString("model")
		cwd, _ := os.Getwd()

		ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-flag-cleanup")
		if err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}

		// 3. Process files
		for _, file := range files {
			fmt.Fprintf(cmd.OutOrStdout(), "🤖 Processing %s...\n", file)
			content, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", file, err)
			}

			prompt := fmt.Sprintf(`Refactor the following Go code to REMOVE the feature flag "%s".
Assume the feature is graduating, so the code guarded by the flag should be KEPT, and the "else" block (if any) should be REMOVED.
Do not wrap the output in markdown code blocks. Return ONLY the raw file content.
If the flag is not actually used in logic (just a string), leave it as is or do your best.

File: %s
Content:
%s`, flagName, file, string(content))

			newContent, err := ag.Send(ctx, prompt)
			if err != nil {
				return fmt.Errorf("agent failed on %s: %w", file, err)
			}

			// Sanitize output (remove markdown blocks if agent ignored instruction)
			newContent = strings.TrimPrefix(newContent, "```go")
			newContent = strings.TrimPrefix(newContent, "```")
			newContent = strings.TrimSuffix(newContent, "```")
			newContent = strings.TrimSpace(newContent) + "\n" // Ensure trailing newline

			// Validate and Format Code
			formatted, err := format.Source([]byte(newContent))
			if err != nil {
				return fmt.Errorf("agent returned invalid Go code for %s: %w\nOutput was:\n%s", file, err, newContent)
			}

			if err := os.WriteFile(file, formatted, 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", file, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✅ Refactored and formatted %s\n", file)
		}

		// 4. Remove from config
		return removeFlagFromConfig(cmd, flagName)
	},
}

func findFilesWithContent(query string) ([]string, error) {
	var matches []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Don't skip the root directory itself if it's "."
			if path == "." {
				return nil
			}
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip non-code files (heuristic)
		if !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".js") && !strings.HasSuffix(path, ".ts") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Ignore read errors
		}
		if strings.Contains(string(content), query) {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

func removeFlagFromConfig(cmd *cobra.Command, name string) error {
	// Viper doesn't have a Delete method for map keys easily.
	// We read the map, delete locally, then set it back.
	flags := viper.GetStringMap("feature_flags")
	delete(flags, name)
	viper.Set("feature_flags", flags)

	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to remove flag from config: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "✅ Flag '%s' removed from configuration.\n", name)
	return nil
}
