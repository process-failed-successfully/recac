package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/jira"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 10, "hello w..."},
		{"abc", 3, "abc"},
		{"abcd", 3, "..."},
	}

	for _, tt := range tests {
		actual := truncateString(tt.input, tt.maxLen)
		assert.Equal(t, tt.expected, actual, "truncateString(%q, %d)", tt.input, tt.maxLen)
	}
}

func TestJiraCleanupCmd_NoLabel(t *testing.T) {
	cmd := &cobra.Command{Use: "cleanup"}
	cmd.Flags().String("label", "", "")

	var exitCode int
	originalExit := exit
	exit = func(code int) {
		exitCode = code
		panic("exit")
	}
	defer func() { exit = originalExit }()

	defer func() {
		r := recover()
		assert.NotNil(t, r, "Expected panic with 'exit'")
		if r != nil {
			assert.Equal(t, "exit", r)
			assert.Equal(t, 1, exitCode)
		}
	}()

	jiraCleanupCmd.Run(cmd, []string{})
}

func TestJiraCleanupCmd_ClientError(t *testing.T) {
	cmd := &cobra.Command{Use: "cleanup"}
	cmd.Flags().String("label", "test-label", "")
	cmd.Flags().Set("label", "test-label")

	originalGetJiraClient := cmdutils.GetJiraClient
	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return nil, fmt.Errorf("mock client error")
	}
	defer func() { cmdutils.GetJiraClient = originalGetJiraClient }()

	var exitCode int
	originalExit := exit
	exit = func(code int) {
		exitCode = code
		panic("exit")
	}
	defer func() { exit = originalExit }()

	defer func() {
		r := recover()
		assert.NotNil(t, r, "Expected panic with 'exit'")
		if r != nil {
			assert.Equal(t, "exit", r)
			assert.Equal(t, 1, exitCode)
		}
	}()

	jiraCleanupCmd.Run(cmd, []string{})
}

func TestRunGenerateTicketsCmd_NoSpecFile(t *testing.T) {
	cmd := &cobra.Command{Use: "generate"}
	cmd.Flags().String("spec", "non_existent_spec_file_test.txt", "")
	cmd.Flags().Set("spec", "non_existent_spec_file_test.txt")

	var exitCode int
	originalExit := exit
	exit = func(code int) {
		exitCode = code
		panic("testing_exit")
	}
	defer func() { exit = originalExit }()

	defer func() {
		r := recover()
		assert.NotNil(t, r, "Expected panic with 'testing_exit'")
		if r != nil {
			assert.Equal(t, "testing_exit", r)
			assert.Equal(t, 1, exitCode)
		}
	}()

	runGenerateTicketsCmd(cmd, []string{})
}

func TestRunGenerateFromArchCmd(t *testing.T) {
	tests := []struct {
		name          string
		archContent   string
		specContent   string
		expectedKey   string
		expectedValue string
	}{
		{
			name: "Success path with valid arch and spec",
			archContent: `
system_name: TestSystem
components:
  - id: COMP-1
    type: Service
    description: Test Component
    implementation_steps:
      - Step 1
      - Step 2
    functions:
      - name: testFunc
        args: string
        return: error
        description: A test func
        requirements:
          - Req 1
    consumes:
      - source: external
        protocol: http
        format: json
        type: InputType
        description: API input
    produces:
      - target: db
        protocol: tcp
        format: sql
        type: OutputType
        description: DB output
`,
			specContent:   "Test Spec",
			expectedKey:   "SYSTEM",
			expectedValue: "TEST-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock Jira server
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(map[string]string{
					"id":  "10000",
					"key": "TEST-1",
				})
			}))
			defer ts.Close()

			// Mock viper settings
			viper.Set("jira.url", ts.URL)
			viper.Set("jira.username", "testuser")
			viper.Set("jira.api_token", "testtoken")
			viper.Set("jira.project_key", "TEST")
			defer viper.Reset()

			// Create a dummy arch file
			archPath := filepath.Join(t.TempDir(), "test_arch.yaml")
			err := os.WriteFile(archPath, []byte(tt.archContent), 0644)
			assert.NoError(t, err)

			// Create a dummy spec file
			specPath := filepath.Join(t.TempDir(), "test_spec.txt")
			err = os.WriteFile(specPath, []byte(tt.specContent), 0644)
			assert.NoError(t, err)

			outPath := filepath.Join(t.TempDir(), "test_jira_arch_out.json")

			// Setup CLI command
			cmd := &cobra.Command{Use: "generate-from-arch"}
			cmd.Flags().String("arch", archPath, "")
			cmd.Flags().Set("arch", archPath)
			cmd.Flags().String("spec", specPath, "")
			cmd.Flags().Set("spec", specPath)
			cmd.Flags().String("project", "TEST", "")
			cmd.Flags().Set("project", "TEST")
			cmd.Flags().StringSlice("label", []string{"test-label"}, "")
			cmd.Flags().Set("label", "test-label")
			cmd.Flags().String("repo-url", "https://github.com/example/repo", "")
			cmd.Flags().Set("repo-url", "https://github.com/example/repo")
			cmd.Flags().String("output-json", outPath, "")
			cmd.Flags().Set("output-json", outPath)

			// Override exit to catch panics
			var exitCode int = -1
			originalExit := exit
			exit = func(code int) {
				exitCode = code
				panic("testing_exit")
			}
			defer func() { exit = originalExit }()

			func() {
				defer func() {
					r := recover()
					if r != nil && r != "testing_exit" {
						panic(r)
					}
				}()
				runGenerateFromArchCmd(cmd, []string{})
			}()

			assert.Equal(t, -1, exitCode, "Expected successful execution, but exit was called with code %d", exitCode)

			// Verify output JSON file
			outputData, err := os.ReadFile(outPath)
			assert.NoError(t, err)
			var output map[string]string
			err = json.Unmarshal(outputData, &output)
			assert.NoError(t, err)
			assert.NotEmpty(t, output)
			assert.Equal(t, tt.expectedValue, output[tt.expectedKey])
		})
	}
}

func TestRunGenerateTicketsCmd(t *testing.T) {
	tests := []struct {
		name          string
		specContent   string
		agentResponse string
		expectedKey   string
		expectedValue string
	}{
		{
			name:        "Success path with valid spec and mapped ID",
			specContent: "Epic: Test Epic\nStory: Test Story",
			agentResponse: "```json\n[\n  {\n    \"title\": \"ID:[TEST-EPIC] Epic: Test Epic\",\n    \"description\": \"Repo: https://github.com/example/repo\\nTest Epic desc\",\n    \"type\": \"Epic\",\n    \"children\": []\n  }\n]\n```",
			expectedKey:   "TEST-EPIC",
			expectedValue: "TEST-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock Jira server
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(map[string]string{
					"id":  "10000",
					"key": "TEST-1",
				})
			}))
			defer ts.Close()

			// Mock viper settings
			viper.Set("jira.url", ts.URL)
			viper.Set("jira.username", "testuser")
			viper.Set("jira.api_token", "testtoken")
			viper.Set("jira.project_key", "TEST")
			defer viper.Reset()

			// Create a dummy spec file
			specPath := filepath.Join(t.TempDir(), "test_app_spec.txt")
			err := os.WriteFile(specPath, []byte(tt.specContent), 0644)
			assert.NoError(t, err)

			outPath := filepath.Join(t.TempDir(), "test_jira_out.json")

			// Setup CLI command
			cmd := &cobra.Command{Use: "generate"}
			cmd.Flags().String("spec", specPath, "")
			cmd.Flags().Set("spec", specPath)
			cmd.Flags().String("project", "TEST", "")
			cmd.Flags().Set("project", "TEST")
			cmd.Flags().String("provider", "mock", "")
			cmd.Flags().Set("provider", "mock")
			cmd.Flags().String("model", "mock", "")
			cmd.Flags().Set("model", "mock")
			cmd.Flags().StringSlice("label", []string{"test-label"}, "")
			cmd.Flags().Set("label", "test-label")
			cmd.Flags().String("repo-url", "https://github.com/example/repo", "")
			cmd.Flags().Set("repo-url", "https://github.com/example/repo")
			cmd.Flags().String("output-json", outPath, "")
			cmd.Flags().Set("output-json", outPath)

			// Mock Agent
			origAgentFactory := agentClientFactory
			agentClientFactory = func(ctx context.Context, provider, model, dir, name string) (agent.Agent, error) {
				return &MockRunAgent{
					Response: tt.agentResponse,
				}, nil
			}
			defer func() { agentClientFactory = origAgentFactory }()

			// Override exit to catch panics
			var exitCode int = -1
			originalExit := exit
			exit = func(code int) {
				exitCode = code
				panic("testing_exit")
			}
			defer func() { exit = originalExit }()

			func() {
				defer func() {
					r := recover()
					if r != nil && r != "testing_exit" {
						panic(r)
					}
				}()
				runGenerateTicketsCmd(cmd, []string{})
			}()

			assert.Equal(t, -1, exitCode, "Expected successful execution, but exit was called with code %d", exitCode)

			// Verify output JSON file
			outputData, err := os.ReadFile(outPath)
			assert.NoError(t, err)
			var output map[string]string
			err = json.Unmarshal(outputData, &output)
			assert.NoError(t, err)
			assert.NotEmpty(t, output)
			assert.Equal(t, tt.expectedValue, output[tt.expectedKey])
		})
	}
}

func TestRunGenerateFromArchCmd_NoArchFile(t *testing.T) {
	cmd := &cobra.Command{Use: "generate-from-arch"}
	cmd.Flags().String("arch", "non_existent_arch_file.yaml", "")
	cmd.Flags().Set("arch", "non_existent_arch_file.yaml")

	var exitCode int
	originalExit := exit
	exit = func(code int) {
		exitCode = code
		panic("testing_exit")
	}
	defer func() { exit = originalExit }()

	defer func() {
		r := recover()
		assert.NotNil(t, r, "Expected panic with 'testing_exit'")
		if r != nil {
			assert.Equal(t, "testing_exit", r)
			assert.Equal(t, 1, exitCode)
		}
	}()

	runGenerateFromArchCmd(cmd, []string{})
}

func TestRunGenerateFromArchCmd_ClientInitError(t *testing.T) {
	// Create a dummy arch file
	archPath := "test_arch.yaml"
	archContent := `
system_name: TestSystem
components:
  - id: COMP-1
    type: Service
    description: Test Component
    implementation_steps:
      - Step 1
      - Step 2
    functions:
      - name: testFunc
        args: string
        return: error
        description: A test func
        requirements:
          - Req 1
    consumes:
      - source: external
        protocol: http
        format: json
        description: API input
    produces:
      - target: db
        protocol: tcp
        format: sql
        description: DB output
`
	err := os.WriteFile(archPath, []byte(archContent), 0644)
	assert.NoError(t, err)
	defer os.Remove(archPath)

	// Create a dummy spec file
	specPath := "test_spec.txt"
	err = os.WriteFile(specPath, []byte("Test Spec"), 0644)
	assert.NoError(t, err)
	defer os.Remove(specPath)

	// Mock the CLI command
	cmd := &cobra.Command{Use: "generate-from-arch"}
	cmd.Flags().String("arch", archPath, "")
	cmd.Flags().Set("arch", archPath)
	cmd.Flags().String("spec", specPath, "")
	cmd.Flags().Set("spec", specPath)
	cmd.Flags().String("project", "TEST", "")
	cmd.Flags().Set("project", "TEST")
	cmd.Flags().StringSlice("label", []string{"test-label"}, "")
	cmd.Flags().Set("label", "test-label")
	cmd.Flags().String("repo-url", "https://github.com/example/repo", "")
	cmd.Flags().Set("repo-url", "https://github.com/example/repo")

	outPath := "test_jira_arch_out.json"
	cmd.Flags().String("output-json", outPath, "")
	cmd.Flags().Set("output-json", outPath)
	defer os.Remove(outPath)

	// Note: since GetJiraClient returns a concrete type, we mock an initialization error
	// to cleanly verify the error handling path without panicking.
	originalGetJiraClient := cmdutils.GetJiraClient
	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return nil, fmt.Errorf("mock error to avoid nil pointer panic inside client")
	}
	defer func() { cmdutils.GetJiraClient = originalGetJiraClient }()

	var exitCode int
	originalExit := exit
	exit = func(code int) {
		exitCode = code
		panic("testing_exit")
	}
	defer func() { exit = originalExit }()

	defer func() {
		r := recover()
		assert.NotNil(t, r, "Expected panic with 'testing_exit'")
		if r != nil {
			assert.Equal(t, "testing_exit", r)
			assert.Equal(t, 1, exitCode)
		}
	}()

	runGenerateFromArchCmd(cmd, []string{})
}

func TestRunGenerateTicketsCmd_ClientInitError(t *testing.T) {
	// Create a dummy spec file
	specPath := "test_app_spec.txt"
	err := os.WriteFile(specPath, []byte("Epic: Test Epic\nStory: Test Story"), 0644)
	assert.NoError(t, err)
	defer os.Remove(specPath)

	// Mock the CLI command
	cmd := &cobra.Command{Use: "generate"}
	cmd.Flags().String("spec", specPath, "")
	cmd.Flags().Set("spec", specPath)
	cmd.Flags().String("project", "TEST", "")
	cmd.Flags().Set("project", "TEST")
	cmd.Flags().String("provider", "mock", "")
	cmd.Flags().Set("provider", "mock")
	cmd.Flags().String("model", "mock", "")
	cmd.Flags().Set("model", "mock")
	cmd.Flags().StringSlice("label", []string{"test-label"}, "")
	cmd.Flags().Set("label", "test-label")
	cmd.Flags().String("repo-url", "https://github.com/example/repo", "")
	cmd.Flags().Set("repo-url", "https://github.com/example/repo")

	outPath := "test_jira_out.json"
	cmd.Flags().String("output-json", outPath, "")
	cmd.Flags().Set("output-json", outPath)
	defer os.Remove(outPath)

	// Note: since GetJiraClient returns a concrete type, we mock an initialization error
	// to cleanly verify the error handling path without panicking.
	originalGetJiraClient := cmdutils.GetJiraClient
	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return nil, fmt.Errorf("mock error to avoid nil pointer panic inside client")
	}
	defer func() { cmdutils.GetJiraClient = originalGetJiraClient }()

	// Mock Agent
	origAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, dir, name string) (agent.Agent, error) {
		return &MockRunAgent{
			Response: "```json\n[\n  {\n    \"title\": \"Epic: Test Epic\",\n    \"description\": \"Test Epic desc\",\n    \"type\": \"Epic\",\n    \"children\": [\n      {\n        \"title\": \"Story: Test Story\",\n        \"description\": \"Test Story desc\",\n        \"type\": \"Story\",\n        \"children\": []\n      }\n    ]\n  }\n]\n```",
		}, nil
	}
	defer func() { agentClientFactory = origAgentFactory }()

	var exitCode int
	originalExit := exit
	exit = func(code int) {
		exitCode = code
		panic("testing_exit")
	}
	defer func() { exit = originalExit }()

	defer func() {
		r := recover()
		assert.NotNil(t, r, "Expected panic with 'testing_exit'")
		if r != nil {
			assert.Equal(t, "testing_exit", r)
			assert.Equal(t, 1, exitCode)
		}
	}()

	runGenerateTicketsCmd(cmd, []string{})
}
