package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestSettingsCmd(t *testing.T) {
	// Create a temporary directory for the config file
	tmpDir, err := os.MkdirTemp("", "recac-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a temporary config file
	configFile := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configFile, []byte("key1: value1\nkey2: value2\n"), 0644)
	assert.NoError(t, err)

	// Set Viper to use the temporary config file
	viper.SetConfigFile(configFile)
	err = viper.ReadInConfig()
	assert.NoError(t, err)

	t.Run("get", func(t *testing.T) {
		t.Run("existing key", func(t *testing.T) {
			var out bytes.Buffer
			settingsGetCmd.SetOut(&out)

			err := settingsGetCmd.RunE(settingsGetCmd, []string{"key1"})
			assert.NoError(t, err)
			assert.Equal(t, "value1\n", out.String())
		})

		t.Run("non-existing key", func(t *testing.T) {
			err := settingsGetCmd.RunE(settingsGetCmd, []string{"nonexistent"})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "key not found")
		})
	})

	t.Run("set", func(t *testing.T) {
		var out bytes.Buffer
		settingsSetCmd.SetOut(&out)

		err := settingsSetCmd.RunE(settingsSetCmd, []string{"key3", "value3"})
		assert.NoError(t, err)
		assert.Equal(t, "Set key3 = value3\n", out.String())

		// Verify the value was set in Viper
		assert.Equal(t, "value3", viper.GetString("key3"))

		// Verify the config file was updated
		content, err := os.ReadFile(configFile)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "key3: value3")
	})

	t.Run("view", func(t *testing.T) {
		var out bytes.Buffer
		settingsViewCmd.SetOut(&out)

		err := settingsViewCmd.RunE(settingsViewCmd, []string{})
		assert.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "key1: value1")
		assert.Contains(t, output, "key2: value2")
		assert.Contains(t, output, "key3: value3")
	})

	t.Run("edit", func(t *testing.T) {
		t.Run("with file", func(t *testing.T) {
			// Mock execCommand
			origExec := execCommand
			var executedCmd string
			var executedArgs []string
			execCommand = func(name string, arg ...string) *exec.Cmd {
				executedCmd = name
				executedArgs = arg
				// Use TestHelperProcess setup if we need to mock it effectively without actually opening editor
				return exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", name)
			}
			defer func() { execCommand = origExec }()

			// Set mock editor
			os.Setenv("EDITOR", "mockeditor")
			defer os.Unsetenv("EDITOR")

			// Need to mock viperConfigFileUsed because in test viper might not return the right string directly
			// Or we just rely on viper.ConfigFileUsed() which we know we set
			origViperConfigUsed := viperConfigFileUsed
			viperConfigFileUsed = func() string { return configFile }
			defer func() { viperConfigFileUsed = origViperConfigUsed }()

			var out bytes.Buffer
			settingsEditCmd.SetOut(&out)

			err := settingsEditCmd.RunE(settingsEditCmd, []string{})
			assert.NoError(t, err)

			assert.Equal(t, "mockeditor", executedCmd)
			assert.Equal(t, []string{configFile}, executedArgs)
			assert.Contains(t, out.String(), "Configuration saved")
		})

		t.Run("no file", func(t *testing.T) {
			origViperConfigUsed := viperConfigFileUsed
			viperConfigFileUsed = func() string { return "" }
			defer func() { viperConfigFileUsed = origViperConfigUsed }()

			err := settingsEditCmd.RunE(settingsEditCmd, []string{})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "no configuration file found")
		})

		t.Run("default editor", func(t *testing.T) {
			origExec := execCommand
			var executedCmd string
			execCommand = func(name string, arg ...string) *exec.Cmd {
				executedCmd = name
				return exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", name)
			}
			defer func() { execCommand = origExec }()

			os.Unsetenv("EDITOR")

			origViperConfigUsed := viperConfigFileUsed
			viperConfigFileUsed = func() string { return configFile }
			defer func() { viperConfigFileUsed = origViperConfigUsed }()

			var out bytes.Buffer
			settingsEditCmd.SetOut(&out)

			err := settingsEditCmd.RunE(settingsEditCmd, []string{})
			assert.NoError(t, err)

			assert.Equal(t, "vim", executedCmd)
		})
	})
}
