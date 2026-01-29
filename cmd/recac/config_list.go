package main

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"recac/internal/agent"
)

// listKeys lists all the configuration keys and their values.
func listKeys(cmd *cobra.Command, args []string) error {
	keys := viper.AllKeys()
	sort.Strings(keys)

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "KEY\tVALUE")
	fmt.Fprintln(w, "---\t-----")

	for _, key := range keys {
		value := viper.Get(key)
		if isSensitive(key) {
			value = "[REDACTED]"
		}
		fmt.Fprintf(w, "%s\t%v\n", key, value)
	}
	return nil
}

// listModels lists all available models, grouped by provider.
func listModels(cmd *cobra.Command, args []string) error {
	agentModels := agent.LoadAllModels()
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Sort providers for consistent output
	providers := make([]string, 0, len(agentModels))
	for provider := range agentModels {
		providers = append(providers, provider)
	}
	sort.Strings(providers)

	for _, provider := range providers {
		title := provider
		if len(title) > 0 {
			title = strings.ToUpper(title[:1]) + title[1:]
		}
		fmt.Fprintf(w, "Provider: %s\n", title)
		fmt.Fprintln(w, "  NAME\tMODEL ID\tDESCRIPTION")
		fmt.Fprintln(w, "  ----\t--------\t-----------")

		models := agentModels[provider]
		for _, model := range models {
			fmt.Fprintf(w, "  %s\t%s\t%s\n", model.Name, model.Value, model.DescriptionDetails)
		}
		fmt.Fprintln(w) // Add a blank line between providers
	}

	return nil
}

// isSensitive checks if a key is sensitive.
func isSensitive(key string) bool {
	lowerKey := strings.ToLower(key)
	return strings.Contains(lowerKey, "key") ||
		strings.Contains(lowerKey, "token") ||
		strings.Contains(lowerKey, "secret")
}
