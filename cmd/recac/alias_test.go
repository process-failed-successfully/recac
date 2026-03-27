package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestAliasCommands(t *testing.T) {
	// Setup temporary config file
	tmpConfigFile := "test_config_alias.yaml"
	f, _ := os.Create(tmpConfigFile)
	f.Close()
	defer os.Remove(tmpConfigFile)

	viper.SetConfigFile(tmpConfigFile)
	viper.Set("aliases", map[string]string{})

	// Set dummy API key to satisfy config validation
	viper.Set("api_key", "dummy-key-for-test")

	// Helper to execute command via rootCmd
	execute := func(args ...string) (string, error) {
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		// We use the full command path and force config file
		fullArgs := append([]string{"--config", tmpConfigFile, "alias"}, args...)
		rootCmd.SetArgs(fullArgs)
		err := rootCmd.Execute()
		return buf.String(), err
	}

	// Test Set
	t.Run("Set Alias", func(t *testing.T) {
		out, err := execute("set", "foo", "bar baz")
		assert.NoError(t, err)
		assert.Contains(t, out, "Alias 'foo' set to 'bar baz'")

		aliases := viper.GetStringMapString("aliases")
		assert.Equal(t, "bar baz", aliases["foo"])
	})

	// Test Get
	t.Run("Get Alias", func(t *testing.T) {
		out, err := execute("get", "foo")
		assert.NoError(t, err)
		assert.Contains(t, out, "bar baz")
	})

	// Test List
	t.Run("List Aliases", func(t *testing.T) {
		viper.Set("aliases", map[string]string{"a": "1", "b": "2"})
		out, err := execute("list")
		assert.NoError(t, err)
		assert.Contains(t, out, "a = 1")
		assert.Contains(t, out, "b = 2")
	})

	// Test Delete
	t.Run("Delete Alias", func(t *testing.T) {
		viper.Set("aliases", map[string]string{"foo": "bar"})
		out, err := execute("delete", "foo")
		assert.NoError(t, err)
		assert.Contains(t, out, "deleted")

		val := viper.GetString("aliases.foo")
		assert.Empty(t, val)
	})

	// Cleanup
	viper.Reset()
}

func TestRegisterAliasCommands(t *testing.T) {
	// Setup
	viper.Reset()
	viper.Set("aliases", map[string]string{
		"testalias": "version", // Use a simple safe command
	})
	// Set dummy API key to satisfy config validation
	viper.Set("api_key", "dummy-key-for-test")

	// Register
	registerAliasCommands()

	// Verify command exists in rootCmd
	cmd, _, err := rootCmd.Find([]string{"testalias"})
	assert.NoError(t, err)
	assert.Equal(t, "testalias", cmd.Name())
	assert.Equal(t, "Alias for 'version'", cmd.Short)

	// Verify executing alias works
	// We don't need to actually test command output here if it's printing to
	// standard out rather than cobra's OutOrStdout, just testing execution works
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"testalias"})
	err = rootCmd.Execute()
	assert.NoError(t, err)

	// Clean up: remove the command to avoid polluting other tests
	rootCmd.RemoveCommand(cmd)
}

func TestRegisterAliasCommands_Invalid(t *testing.T) {
	viper.Reset()
	// An alias with an unclosed quote to fail shellquote.Split
	viper.Set("aliases", map[string]string{
		"badalias": "echo 'unclosed",
	})

	registerAliasCommands()

	cmd, _, err := rootCmd.Find([]string{"badalias"})
	assert.NoError(t, err)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"badalias"})
	err = rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid alias configuration for 'badalias'")

	rootCmd.RemoveCommand(cmd)
}

func TestRegisterAliasCommands_OverrideSkip(t *testing.T) {
	viper.Reset()
	// Try to register an alias over an existing command, e.g., 'version'
	viper.Set("aliases", map[string]string{
		"version": "echo fake",
	})

	registerAliasCommands()

	_, _, err := rootCmd.Find([]string{"version"})
	assert.NoError(t, err)

	// Verify it's still the original version command, not our alias
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"version"})
	err = rootCmd.Execute()
	assert.NoError(t, err)
	assert.NotContains(t, buf.String(), "fake") // It should run the real version command
}

func TestCannotOverrideBuiltin(t *testing.T) {
	// Helper to execute command via rootCmd
	execute := func(args ...string) (string, error) {
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		// We don't strictly need the temp config here but good practice
		fullArgs := append([]string{"alias"}, args...)
		rootCmd.SetArgs(fullArgs)
		// Set dummy API key to satisfy config validation
		viper.Set("api_key", "dummy-key-for-test")
		err := rootCmd.Execute()
		return buf.String(), err
	}

	// Try to set alias named "todo" (existing command)
	_, err := execute("set", "todo", "echo fail")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot override existing command")
}
