package main

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for configuration keys by substring",
	Long: `Search for configuration keys that contain the given substring (case-insensitive).
This is useful when you want to find a configuration option but don't remember its exact name.

It prints the matching keys and their current values (redacting sensitive keys).`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.ToLower(args[0])
		keys := viper.AllKeys()
		sort.Strings(keys)

		var matchingKeys []string
		for _, key := range keys {
			if strings.Contains(strings.ToLower(key), query) {
				matchingKeys = append(matchingKeys, key)
			}
		}

		if len(matchingKeys) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No configuration keys found matching %q.\n", args[0])
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		defer w.Flush()

		fmt.Fprintln(w, "KEY\tVALUE")
		fmt.Fprintln(w, "---\t-----")

		for _, key := range matchingKeys {
			value := viper.Get(key)
			if isSensitive(key) {
				value = "[REDACTED]"
			}
			fmt.Fprintf(w, "%s\t%v\n", key, value)
		}

		return nil
	},
}
