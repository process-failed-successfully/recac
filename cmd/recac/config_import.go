package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var configImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import configuration from a file",
	Long: `Import configuration from a YAML or JSON file.

This will merge the settings from the specified file into your current configuration.
Existing keys will be overwritten with the values from the imported file.

Examples:
  recac config import backup.yaml
  recac config import new-settings.json`,
	Args: cobra.ExactArgs(1),
	RunE: importConfig,
}

func importConfig(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	// Read the file
	data, err := readFileFunc(filePath)
	if err != nil {
		return fmt.Errorf("failed to read import file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	var importData map[string]interface{}

	if ext == ".json" {
		if err := json.Unmarshal(data, &importData); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
	} else if ext == ".yaml" || ext == ".yml" {
		if err := yaml.Unmarshal(data, &importData); err != nil {
			return fmt.Errorf("failed to parse YAML: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported file extension: %s. Please provide a .yaml or .json file", ext)
	}

	// Flatten and apply the settings
	flatData := flattenMap(importData, "")
	for k, v := range flatData {
		viper.Set(k, v)
	}

	cfgFile := viperConfigFileUsed()
	if cfgFile == "" {
		cfgFile = "config.yaml"
	}

	// Write the configuration back
	err = viperSafeWriteConfig()
	if err != nil {
		err = viper.WriteConfigAs(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to write config to %s: %w", cfgFile, err)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Successfully imported %d keys from %s\n", len(flatData), filePath)
	return nil
}

// flattenMap converts a nested map into a single-level map with dot-separated keys
func flattenMap(m map[string]interface{}, prefix string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		switch child := v.(type) {
		case map[string]interface{}:
			childMap := flattenMap(child, key)
			for ck, cv := range childMap {
				result[ck] = cv
			}
		// yaml.v3 unmarshals nested objects as map[interface{}]interface{}
		case map[interface{}]interface{}:
			strMap := make(map[string]interface{})
			for mk, mv := range child {
				strMap[fmt.Sprintf("%v", mk)] = mv
			}
			childMap := flattenMap(strMap, key)
			for ck, cv := range childMap {
				result[ck] = cv
			}
		default:
			result[key] = v
		}
	}
	return result
}

func init() {
	configCmd.AddCommand(configImportCmd)
}
