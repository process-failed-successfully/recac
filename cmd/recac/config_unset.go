package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var unsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: "Unset a configuration value",
	Long: `Unset a configuration value for the recac CLI.

This removes the specified key from the configuration file.

Examples:
  recac config unset agent.provider
  recac config unset max_iterations`,
	Args: cobra.ExactArgs(1),
	RunE: unsetConfigKey,
}

func unsetConfigKey(cmd *cobra.Command, args []string) error {
	key := args[0]

	cfgFile := viperConfigFileUsed()
	if cfgFile == "" {
		cfgFile = "config.yaml"
	}

	data, err := readFileFunc(cfgFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config file %s does not exist", cfgFile)
		}
		return fmt.Errorf("failed to read config file %s: %w", cfgFile, err)
	}

	var configMap map[string]interface{}
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		return fmt.Errorf("failed to parse config file %s: %w", cfgFile, err)
	}

	keys := strings.Split(key, ".")
	if !deleteNestedKey(configMap, keys) {
		fmt.Fprintf(cmd.OutOrStdout(), "Key %s not found in configuration\n", key)
		return nil
	}

	newData, err := yaml.Marshal(&configMap)
	if err != nil {
		return fmt.Errorf("failed to marshal config file %s: %w", cfgFile, err)
	}

	if err := writeFileFunc(cfgFile, newData, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", cfgFile, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Successfully unset %s\n", key)
	return nil
}

func deleteNestedKey(m map[string]interface{}, keys []string) bool {
	if len(keys) == 0 {
		return false
	}

	key := keys[0]

	if len(keys) == 1 {
		if _, exists := m[key]; exists {
			delete(m, key)
			return true
		}
		return false
	}

	if nextMap, ok := m[key].(map[string]interface{}); ok {
		deleted := deleteNestedKey(nextMap, keys[1:])
		if deleted && len(nextMap) == 0 {
			delete(m, key)
		}
		return deleted
	} else if nextMapInterface, ok := m[key].(map[interface{}]interface{}); ok {
		nextMapStr := make(map[string]interface{})
		for k, v := range nextMapInterface {
			if strKey, ok := k.(string); ok {
				nextMapStr[strKey] = v
			}
		}
		deleted := deleteNestedKey(nextMapStr, keys[1:])
		m[key] = nextMapStr
		if deleted && len(nextMapStr) == 0 {
			delete(m, key)
		}
		return deleted
	}

	return false
}
