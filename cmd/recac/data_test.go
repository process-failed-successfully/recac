package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// DataMockAgent is a unique mock for data_test.go to avoid conflicts
type DataMockAgent struct {
	mock.Mock
}

func (m *DataMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *DataMockAgent) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	args := m.Called(ctx, prompt, callback)
	return args.String(0), args.Error(1)
}

func TestDataCmd(t *testing.T) {
	// Mock agent factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := new(DataMockAgent)
	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return mockAgent, nil
	}

	tests := []struct {
		name        string
		args        []string
		mockResp    string
		expectError bool
		checkOut    func(t *testing.T, output string)
	}{
		{
			name:     "JSON Generation",
			args:     []string{"--schema", "id:int", "--count", "3", "--format", "json"},
			mockResp: `[{"id": 1}, {"id": 2}, {"id": 3}]`,
			checkOut: func(t *testing.T, output string) {
				assert.Contains(t, output, `[{"id": 1}, {"id": 2}, {"id": 3}]`)
			},
		},
		{
			name:     "CSV Generation with Desc",
			args:     []string{"--desc", "users", "--format", "csv"},
			mockResp: "id,name\n1,alice",
			checkOut: func(t *testing.T, output string) {
				assert.Contains(t, output, "id,name\n1,alice")
			},
		},
		{
			name:     "Invalid Args",
			args:     []string{}, // No schema or desc
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags (Cobra flags persist in package globals if using the shared cmd variable)
			// But here we invoke the RunE function directly or use a new command wrapper?
			// The flags are defined in `init()` on `dataCmd`.
			// `dataCmd.SetArgs` can help, but since we modify global vars `dataSchema` etc in `init`,
			// we must be careful.
			// Ideally, we should reset the global vars manually.
			dataSchema = ""
			dataDesc = ""
			dataCount = 5
			dataFormat = "json"
			dataOut = ""

			// Set up mock
			if !tt.expectError {
				mockAgent.On("Send", mock.Anything, mock.Anything).Return(tt.mockResp, nil).Once()
			}

			// Capture output
			buf := new(bytes.Buffer)

			// We can't reuse `dataCmd` easily because of global flags accumulation if not careful.
			// But `ExecuteC` or `SetArgs` + `Execute` is standard.
			// Let's rely on Cobra to parse into the global vars.

			// Important: Cobra flags parsing writes to the variables bound in `init()`.
			// We need to call dataCmd.ParseFlags(tt.args) but Execute does that.

			// To isolate, we can reset flags? No, `init` ran once.
			// We will just assume `dataCmd` is the global one.

			dataCmd.SetOut(buf)
			dataCmd.SetErr(buf)
			dataCmd.SetArgs(tt.args)

			// We need to silence usage on error to keep logs clean
			rootCmd.SilenceUsage = true
			rootCmd.SilenceErrors = true

			// Use rootCmd to ensure proper parsing
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)
			rootCmd.SetArgs(append([]string{"data"}, tt.args...))

			err := rootCmd.Execute()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkOut != nil {
					tt.checkOut(t, buf.String())
				}

				// Verify prompt
				mockAgent.AssertExpectations(t)
			}
		})
	}
}

func TestDataCmd_File(t *testing.T) {
	// Mock agent factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := new(DataMockAgent)
	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return mockAgent, nil
	}

	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "data.json")

	mockResp := `[{"foo": "bar"}]`
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(mockResp, nil)

	// Reset globals
	dataSchema = ""
	dataDesc = ""
	dataCount = 5
	dataFormat = "json"
	dataOut = ""

	rootCmd.SetArgs([]string{"data", "--desc", "test", "--out", outFile})
	rootCmd.SetOut(new(bytes.Buffer))

	err := rootCmd.Execute()
	assert.NoError(t, err)

	// Check file
	content, err := os.ReadFile(outFile)
	assert.NoError(t, err)
	assert.Equal(t, mockResp, string(content))
}
