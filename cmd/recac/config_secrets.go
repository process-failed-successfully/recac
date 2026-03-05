package main

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configSecretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "List known configuration keys that are considered sensitive and their status",
	Long:  "List known configuration keys (those currently registered or set via env/file) that are considered sensitive and their status. This helps to quickly identify what sensitive information is being handled by the CLI.",
	RunE:  runConfigSecrets,
}

func runConfigSecrets(cmd *cobra.Command, args []string) error {
	keys := viper.AllKeys()
	sort.Strings(keys)

	var sensitiveKeys []string
	for _, key := range keys {
		if isSensitive(key) {
			sensitiveKeys = append(sensitiveKeys, key)
		}
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "SENSITIVE KEY\tSTATUS")
	fmt.Fprintln(w, "-------------\t------")

	for _, key := range sensitiveKeys {
		val := viper.Get(key)
		status := "Not Set"
		if val != nil && val != "" {
			status = "Set"
		}
		fmt.Fprintf(w, "%s\t%s\n", key, status)
	}

	return nil
}
