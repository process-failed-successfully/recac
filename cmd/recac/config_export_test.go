package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConfigExportCmd_DefaultYaml(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("agent.provider", "gemini")
	viper.Set("api_key", "secret123")
	viper.Set("timeout", 60)

	output, err := executeCommand(rootCmd, "config", "export")
	require.NoError(t, err)

	// Check YAML format
	assert.Contains(t, output, "agent:")
	assert.Contains(t, output, "provider: gemini")
	assert.Contains(t, output, "timeout: 60")
	assert.Contains(t, output, "api_key: '[REDACTED]'")
	assert.NotContains(t, output, "secret123")
}

func TestConfigExportCmd_Json(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("agent.provider", "gemini")
	viper.Set("api_key", "secret123")
	viper.Set("timeout", 60)

	output, err := executeCommand(rootCmd, "config", "export", "--format", "json")
	require.NoError(t, err)

	// Verify valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(output), &parsed)
	require.NoError(t, err)

	// Check values
	agentMap, ok := parsed["agent"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "gemini", agentMap["provider"])

	// JSON unmarshals numbers to float64 by default
	assert.Equal(t, float64(60), parsed["timeout"])
	assert.Equal(t, "[REDACTED]", parsed["api_key"])
	assert.NotContains(t, output, "secret123")
}

func TestConfigExportCmd_ShowSensitive(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("agent.provider", "gemini")
	viper.Set("api_key", "secret123")

	output, err := executeCommand(rootCmd, "config", "export", "--show-sensitive")
	require.NoError(t, err)

	assert.Contains(t, output, "api_key: secret123")
}

func TestConfigExportCmd_OutputToFile(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("my_data", "my_value")

	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "export.yaml")

	output, err := executeCommand(rootCmd, "config", "export", "--output", outPath)
	require.NoError(t, err)

	assert.Contains(t, output, "Configuration exported to "+outPath)

	contentBytes, err := os.ReadFile(outPath)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = yaml.Unmarshal(contentBytes, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "my_value", parsed["my_data"])
}

func TestRedactSensitiveKeys(t *testing.T) {
	settings := map[string]interface{}{
		"my_data": "value1",
		"api_key": "secret1",
		"nested": map[string]interface{}{
			"token": "secret2",
			"other": "value2",
			"deep": map[string]interface{}{
				"secret_val": "secret3",
			},
		},
	}

	redactSensitiveKeys(settings)

	assert.Equal(t, "value1", settings["my_data"])
	assert.Equal(t, "[REDACTED]", settings["api_key"])

	nested := settings["nested"].(map[string]interface{})
	assert.Equal(t, "[REDACTED]", nested["token"])
	assert.Equal(t, "value2", nested["other"])

	deep := nested["deep"].(map[string]interface{})
	assert.Equal(t, "[REDACTED]", deep["secret_val"])
}
