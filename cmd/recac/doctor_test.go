package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestDoctorCmd_AllPassing(t *testing.T) {
	// Arrange
	originalGoCheck := runCheckGoVersion
	originalDockerCheck := runCheckDockerConnection
	originalConfigCheck := runCheckAppConfig
	originalJiraCheck := runCheckJiraAuth
	originalAICheck := runCheckAIProvider
	defer func() {
		runCheckGoVersion = originalGoCheck
		runCheckDockerConnection = originalDockerCheck
		runCheckAppConfig = originalConfigCheck
		runCheckJiraAuth = originalJiraCheck
		runCheckAIProvider = originalAICheck
	}()

	runCheckGoVersion = func() (string, error) { return "Go is ready", nil }
	runCheckDockerConnection = func() (string, error) { return "Docker is ready", nil }
	runCheckAppConfig = func() (string, error) { return "Config is ready", nil }
	runCheckJiraAuth = func() (string, error) { return "Jira is ready", nil }
	runCheckAIProvider = func() (string, error) { return "AI is ready", nil }

	// Act
	output, err := executeCommand(rootCmd, "doctor")

	// Assert
	assert.NoError(t, err)
	assert.Contains(t, output, "All checks passed!")
	assert.Contains(t, output, "Go is ready")
	assert.Contains(t, output, "Docker is ready")
	assert.Contains(t, output, "Config is ready")
	assert.Contains(t, output, "Jira is ready")
	assert.Contains(t, output, "AI is ready")
}

func TestDoctorCmd_OneFailing(t *testing.T) {
	// Arrange
	originalGoCheck := runCheckGoVersion
	originalDockerCheck := runCheckDockerConnection
	defer func() {
		runCheckGoVersion = originalGoCheck
		runCheckDockerConnection = originalDockerCheck
	}()

	runCheckGoVersion = func() (string, error) { return "Go is ready", nil }
	runCheckDockerConnection = func() (string, error) { return "", errors.New("docker daemon is on fire") }

	// Act
	// The command should exit(1), which is caught as a panic by executeCommand
	output, err := executeCommand(rootCmd, "doctor")

	// Assert
	assert.Error(t, err) // executeCommand converts exit(1) to an error
	assert.Contains(t, err.Error(), "exit status 1")

	assert.Contains(t, output, "Found issues:")
	assert.Contains(t, output, "❌ Docker Environment: docker daemon is on fire")
	assert.NotContains(t, output, "All checks passed!")
}

func TestDoctorCmd_AliasWorks(t *testing.T) {
	// Arrange
	runCheckGoVersion = func() (string, error) { return "Go is ready", nil }
	runCheckDockerConnection = func() (string, error) { return "Docker is ready", nil }
	runCheckAppConfig = func() (string, error) { return "Config is ready", nil }
	runCheckJiraAuth = func() (string, error) { return "Jira is ready", nil }
	runCheckAIProvider = func() (string, error) { return "AI is ready", nil }

	// Act
	output, err := executeCommand(rootCmd, "check")

	// Assert
	assert.NoError(t, err)
	assert.Contains(t, output, "Warning: The 'check' command is deprecated")
	assert.Contains(t, output, "All checks passed!")
}

func TestCheckAppConfig_NoFile(t *testing.T) {
	// Arrange
	originalConfigFile := viper.ConfigFileUsed
	viper.SetConfigFile("/tmp/no_file_here.yaml")
	defer func() {
		viper.SetConfigFile(originalConfigFile())
	}()

	// Act
	msg, err := checkAppConfig()

	// Assert
	assert.Error(t, err)
	assert.Empty(t, msg)
	assert.Contains(t, err.Error(), "does not exist")
}
