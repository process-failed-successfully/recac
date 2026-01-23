package main

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// MockAgentForMockCmd is a mock implementation of agent.Agent
type MockAgentForMockCmd struct {
	Response string
}

func (m *MockAgentForMockCmd) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockAgentForMockCmd) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	callback(m.Response)
	return m.Response, nil
}

func TestMockCmd(t *testing.T) {
	// Setup temporary directory
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "service.go")
	outputFile := filepath.Join(tmpDir, "service_mock.go")

	// Create a dummy Go file with an interface
	sourceContent := `package service

type UserService interface {
	GetUser(id string) (string, error)
	CreateUser(name string) error
}
`
	err := os.WriteFile(sourceFile, []byte(sourceContent), 0644)
	if err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Mock the agent factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	expectedMockContent := `package service

type MockUserService struct {}
func (m *MockUserService) GetUser(id string) (string, error) { return "", nil }
func (m *MockUserService) CreateUser(name string) error { return nil }`

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockAgentForMockCmd{Response: expectedMockContent}, nil
	}

	// Reset flags
	mockInterface = ""
	mockOutput = ""
	mockFramework = "std" // explicit to avoid auto logic issues in test

	// Execute command
	// We call runMock directly to avoid Cobra flag parsing complexity in unit test,
	// but we need to set flags manually if we relied on them.
	// However, the command struct uses global variables populated by flags.
	// So we set them above.

	// Since runMock takes cmd and args:
	// args[0] is file path.

	// Set viper config needed for factory
	viper.Set("provider", "mock")
	viper.Set("model", "mock-model")

	// Set the global variables that the flags bind to
	mockInterface = "UserService"
	mockOutput = outputFile

	err = runMock(mockCmd, []string{sourceFile})
	assert.NoError(t, err)

	// Verify output
	content, err := os.ReadFile(outputFile)
	assert.NoError(t, err)
	assert.Equal(t, expectedMockContent, string(content))
}

func TestMockCmd_NoInterface(t *testing.T) {
	// Test behavior when interface is not specified (should use survey, but we probably want to error or pick default if single)

	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "single.go")

	sourceContent := `package single
type SingleInterface interface { Do() }
`
	os.WriteFile(sourceFile, []byte(sourceContent), 0644)

	// Mock agent
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()
	agentClientFactory = func(ctx context.Context, p, m, pp, pn string) (agent.Agent, error) {
		return &MockAgentForMockCmd{Response: "mock content"}, nil
	}

	mockInterface = "" // Clear it
	mockOutput = filepath.Join(tmpDir, "out.go")

	err := runMock(mockCmd, []string{sourceFile})
	assert.NoError(t, err)

	// Should have picked SingleInterface automatically
	assert.FileExists(t, mockOutput)
}
