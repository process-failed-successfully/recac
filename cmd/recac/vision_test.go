package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestVisionCmd(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "test.png")
	err := os.WriteFile(imagePath, []byte("fake image data"), 0644)
	assert.NoError(t, err)

	// Mock Agent Factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return agent.NewMockAgent(), nil
	}

	// Setup Config
	viper.Set("provider", "mock")
	viper.Set("model", "mock-vision")

	// Run Command
	buf := new(bytes.Buffer)
	visionCmd.SetOut(buf)
	visionCmd.SetErr(buf)

	err = visionCmd.RunE(visionCmd, []string{imagePath, "What is this?"})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Analyzing image")
	assert.Contains(t, output, "(Vision)") // Mock response includes this
	assert.Contains(t, output, "test.png")
}

func TestVisionCmd_FileNotFound(t *testing.T) {
	err := visionCmd.RunE(visionCmd, []string{"nonexistent.png"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image file not found")
}
