package main

import (
	"fmt"
	"sort"

	"github.com/kballard/go-shellquote"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(aliasCmd)
	aliasCmd.AddCommand(aliasSetCmd)
	aliasCmd.AddCommand(aliasGetCmd)
	aliasCmd.AddCommand(aliasListCmd)
	aliasCmd.AddCommand(aliasDeleteCmd)
}

var aliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Manage command aliases",
	Long:  `Create, list, and delete aliases for recac commands.`,
}

var aliasSetCmd = &cobra.Command{
	Use:     "set [name] [command]",
	Short:   "Set a new alias",
	Example: `  recac alias set fix-auth "todo solve --file internal/auth/service.go:42"`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		command := args[1]

		// Prevent overriding built-in commands
		// Note: We need to check if the command exists in the root command
		// We use Find to check if the name resolves to a sub-command of root
		foundCmd, _, _ := rootCmd.Find([]string{name})
		// If foundCmd is not root, it means it found a sub-command (like 'alias', 'todo', etc.)
		// But Find also returns root if it matches nothing but the root itself accepts args?
		// Actually, Find searches for the command. If name matches a subcommand, it returns it.
		// If name doesn't match, it returns the command that *would* handle it (root) and the args.
		if foundCmd != rootCmd {
			return fmt.Errorf("cannot override existing command '%s'", name)
		}

		aliases := viper.GetStringMapString("aliases")
		if aliases == nil {
			aliases = make(map[string]string)
		}
		aliases[name] = command
		viper.Set("aliases", aliases)

		// Try to write to config file
		if err := viper.WriteConfig(); err != nil {
			// If write fails, it might be because the file doesn't exist
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				if err := viper.SafeWriteConfig(); err != nil {
					return fmt.Errorf("failed to write config: %w", err)
				}
			} else {
				return fmt.Errorf("failed to write config: %w", err)
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Alias '%s' set to '%s'\n", name, command)
		return nil
	},
}

var aliasGetCmd = &cobra.Command{
	Use:   "get [name]",
	Short: "Get an alias value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		aliases := viper.GetStringMapString("aliases")
		val, ok := aliases[name]
		if !ok || val == "" {
			return fmt.Errorf("alias '%s' not found", name)
		}
		fmt.Fprintln(cmd.OutOrStdout(), val)
		return nil
	},
}

var aliasListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all aliases",
	Run: func(cmd *cobra.Command, args []string) {
		aliases := viper.GetStringMapString("aliases")
		if len(aliases) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No aliases defined.")
			return
		}

		// Sort keys
		keys := make([]string, 0, len(aliases))
		for k := range aliases {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			fmt.Fprintf(cmd.OutOrStdout(), "%s = %s\n", k, aliases[k])
		}
	},
}

var aliasDeleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Delete an alias",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		aliases := viper.GetStringMapString("aliases")
		if _, ok := aliases[name]; !ok {
			return fmt.Errorf("alias '%s' not found", name)
		}
		delete(aliases, name)
		viper.Set("aliases", aliases)

		if err := viper.WriteConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Alias '%s' deleted\n", name)
		return nil
	},
}

// registerAliasCommands reads aliases from config and adds them to rootCmd.
// This should be called before rootCmd.Execute().
func registerAliasCommands() {
	aliases := viper.GetStringMapString("aliases")
	for name, commandStr := range aliases {
		// Capture variables for closure
		aliasName := name
		aliasVal := commandStr

		// Skip if command already exists to prevent accidental overrides or conflicts
		foundCmd, _, _ := rootCmd.Find([]string{aliasName})
		if foundCmd != rootCmd {
			continue
		}

		cmd := &cobra.Command{
			Use:                aliasName,
			Short:              fmt.Sprintf("Alias for '%s'", aliasVal),
			DisableFlagParsing: true, // We pass flags to the target
			RunE: func(c *cobra.Command, args []string) error {
				// Parse alias string into args
				baseArgs, err := shellquote.Split(aliasVal)
				if err != nil {
					return fmt.Errorf("invalid alias configuration for '%s': %w", aliasName, err)
				}

				// Append user provided args
				fullArgs := append(baseArgs, args...)

				// Execute via rootCmd
				// NOTE: We must reset args because we are reusing rootCmd
				rootCmd.SetArgs(fullArgs)
				return rootCmd.Execute()
			},
		}
		rootCmd.AddCommand(cmd)
	}
}
