package main

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Mock FileInfo
type mockFileInfo struct {
	name string
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return 0 }
func (m *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return false }
func (m *mockFileInfo) Sys() interface{}   { return nil }

func TestCheckCmd(t *testing.T) {
	// Backup original functions
	origViperConfigFileUsed := viperConfigFileUsed
	origOsStatFunc := osStatFunc
	origViperSafeWriteConfig := viperSafeWriteConfig
	origLookPath := lookPath
	origExecCommand := execCommand

	defer func() {
		viperConfigFileUsed = origViperConfigFileUsed
		osStatFunc = origOsStatFunc
		viperSafeWriteConfig = origViperSafeWriteConfig
		lookPath = origLookPath
		execCommand = origExecCommand
	}()

	tests := []struct {
		name          string
		mockConfig    string
		mockStatErr   error
		mockLookPath  error
		mockDockerRun bool // true = success, false = failure
		mockWriteErr  error
		fixFlag       bool
		expectedError bool // true = expect non-nil error
	}{
		{
			name:          "All Pass",
			mockConfig:    "config.yaml",
			mockStatErr:   nil,
			mockLookPath:  nil,
			mockDockerRun: true,
			expectedError: false,
		},
		{
			name:          "Config Missing No Fix",
			mockConfig:    "",
			mockStatErr:   os.ErrNotExist,
			mockLookPath:  nil,
			mockDockerRun: true,
			expectedError: true,
		},
		{
			name:          "Config Missing With Fix Success",
			mockConfig:    "",
			mockStatErr:   os.ErrNotExist,
			mockLookPath:  nil,
			mockDockerRun: true,
			fixFlag:       true,
			expectedError: false,
		},
		{
			name:          "Go Missing",
			mockConfig:    "config.yaml",
			mockStatErr:   nil,
			mockLookPath:  fmt.Errorf("not found"),
			mockDockerRun: true,
			expectedError: true,
		},
		{
			name:          "Docker Missing",
			mockConfig:    "config.yaml",
			mockStatErr:   nil,
			mockLookPath:  nil,
			mockDockerRun: false,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixFlag = tt.fixFlag

			// Mocks
			viperConfigFileUsed = func() string { return tt.mockConfig }
			osStatFunc = func(name string) (os.FileInfo, error) {
				if tt.mockStatErr != nil {
					return nil, tt.mockStatErr
				}
				return &mockFileInfo{name: name}, nil
			}
			viperSafeWriteConfig = func() error { return tt.mockWriteErr }
			lookPath = func(file string) (string, error) {
				if tt.mockLookPath != nil {
					return "", tt.mockLookPath
				}
				return "/usr/bin/go", nil
			}

			// Mock Exec Command to return a command that succeeds or fails
			execCommand = func(name string, arg ...string) *exec.Cmd {
				// We use "true" and "false" commands available on linux/unix
				cmdName := "true"
				if !tt.mockDockerRun && name == "docker" {
					cmdName = "false"
				}
				return exec.Command(cmdName)
			}

			// Execute using RunE
			err := checkCmd.RunE(checkCmd, []string{})

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
