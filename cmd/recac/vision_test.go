package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// MockVisionAgent implements agent.VisionAgent
type MockVisionAgent struct {
	Response string
	Err      error
}

func (m *MockVisionAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, m.Err
}

func (m *MockVisionAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if onChunk != nil {
		onChunk(m.Response)
	}
	return m.Response, m.Err
}

func (m *MockVisionAgent) SendImage(ctx context.Context, prompt, imagePath string) (string, error) {
	return "Image Analyzed: " + m.Response, m.Err
}

func TestVisionCmd(t *testing.T) {
	// Create dummy image
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "screenshot.png")
	os.WriteFile(imagePath, []byte("fake"), 0644)

	// Setup Config
	viper.Set("provider", "gemini")
	viper.Set("model", "gemini-pro-vision")

	// Mock Factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockVisionAgent{Response: "Looks good", Err: nil}, nil
	}

	// Capture Output
	cmd := visionCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	// Call RunE directly to avoid Cobra hierarchy issues in testing
	err := cmd.RunE(cmd, []string{imagePath})
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	output := out.String()
	expected := "Image Analyzed: Looks good"

	if !strings.Contains(output, expected) {
		t.Errorf("Expected output to contain %q, got %q", expected, output)
	}
}

func TestVisionCmd_FileNotFound(t *testing.T) {
	cmd := visionCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	// Call RunE directly
	err := cmd.RunE(cmd, []string{"non-existent.png"})
	if err == nil {
		t.Fatal("Expected error for missing file, got nil")
	}
}
