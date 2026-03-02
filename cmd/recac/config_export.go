package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var configExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the fully resolved configuration",
	Long: `Export the application's configuration as fully resolved by the recac CLI.
This includes the current configuration file, environment variables, and defaults.

Sensitive keys are redacted by default. Use --show-sensitive to expose them.
The output format is YAML by default. Use --format json to output JSON.

Examples:
  recac config export
  recac config export --format json
  recac config export --show-sensitive
  recac config export --output config-backup.yaml`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		showSensitive, _ := cmd.Flags().GetBool("show-sensitive")
		outputFile, _ := cmd.Flags().GetString("output")

		settings := viper.AllSettings()

		if !showSensitive {
			redactSensitiveKeys(settings)
		}

		var data []byte
		var err error

		if format == "json" {
			data, err = json.MarshalIndent(settings, "", "  ")
		} else {
			data, err = yaml.Marshal(settings)
		}

		if err != nil {
			return fmt.Errorf("failed to format configuration: %w", err)
		}

		if outputFile != "" {
			if err := writeFileFunc(outputFile, data, 0600); err != nil {
				return fmt.Errorf("failed to write configuration to file: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Configuration exported to %s\n", outputFile)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
		}

		return nil
	},
}

func init() {
	configExportCmd.Flags().StringP("format", "f", "yaml", "Output format (yaml, json)")
	configExportCmd.Flags().Bool("show-sensitive", false, "Do not redact sensitive values")
	configExportCmd.Flags().StringP("output", "o", "", "Output file path (default stdout)")
}

func redactSensitiveKeys(settings map[string]interface{}) {
	for k, v := range settings {
		if isSensitive(k) {
			settings[k] = "[REDACTED]"
			continue
		}

		if nested, ok := v.(map[string]interface{}); ok {
			redactSensitiveKeys(nested)
		}
	}
}
