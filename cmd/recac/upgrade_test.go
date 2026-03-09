package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
    "fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"recac/internal/agent"
)

// MockAgent for testing
type UpgradeMockAgent struct {
	mock.Mock
}

func (m *UpgradeMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *UpgradeMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func (m *UpgradeMockAgent) GetState() interface{} {
	return nil
}

// TestHelperProcessUpgrade is the target of mocked exec.Command calls for upgrade tests.
func TestHelperProcessUpgrade(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	output := os.Getenv("HELPER_OUTPUT")
	if os.Getenv("HELPER_FAIL") == "1" {
		fmt.Fprint(os.Stdout, output)
		os.Exit(1)
	}
	fmt.Fprint(os.Stdout, output)
	os.Exit(0)
}

func helperCommandContext(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcessUpgrade", "--", command}
	cs = append(cs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", "HELPER_OUTPUT="}
	return cmd
}

func TestRunUpgrade_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(t *testing.T)
		execFunc     func(t *testing.T) func(name string, arg ...string) *exec.Cmd
		args         []string
		wantErr      bool
		wantOutput   []string
		upgradeAll   bool
	}{
		{
			name: "No Candidates Table",
			setupFunc: func(t *testing.T) {},
			execFunc: func(t *testing.T) func(name string, arg ...string) *exec.Cmd {
				return func(name string, arg ...string) *exec.Cmd {
					c := helperCommandContext(context.Background(), name, arg...)
					c.Env = []string{"GO_WANT_HELPER_PROCESS=1", "HELPER_OUTPUT="}
					return c
				}
			},
			args:       []string{"upgrade"},
			wantErr:    false,
			wantOutput: []string{"All dependencies are up to date!"},
			upgradeAll: false,
		},
		{
			name: "With Candidates No All Table",
			setupFunc: func(t *testing.T) {
				os.WriteFile("go.mod", []byte("module test"), 0644)
			},
			execFunc: func(t *testing.T) func(name string, arg ...string) *exec.Cmd {
				return func(name string, arg ...string) *exec.Cmd {
					c := helperCommandContext(context.Background(), name, arg...)
					if name == "go" && len(arg) > 0 && arg[0] == "list" {
						jsonOutput := `{"Path": "github.com/stretchr/testify", "Version": "v1.8.0", "Update": {"Version": "v1.9.0"}}`
						c.Env = []string{"GO_WANT_HELPER_PROCESS=1", "HELPER_OUTPUT=" + jsonOutput}
					} else {
						c.Env = []string{"GO_WANT_HELPER_PROCESS=1", "HELPER_OUTPUT="}
					}
					return c
				}
			},
			args:       []string{"upgrade"},
			wantErr:    false,
			wantOutput: []string{"Outdated dependencies found:", "github.com/stretchr/testify (v1.8.0 -> v1.9.0)", "Run with --all to upgrade all of them."},
			upgradeAll: false,
		},
		{
			name: "With Candidates All Success Table",
			setupFunc: func(t *testing.T) {
				os.WriteFile("go.mod", []byte("module test"), 0644)
			},
			execFunc: func(t *testing.T) func(name string, arg ...string) *exec.Cmd {
				return func(name string, arg ...string) *exec.Cmd {
					c := helperCommandContext(context.Background(), name, arg...)

					if name == "go" && len(arg) > 0 && arg[0] == "list" {
						jsonOutput := `{"Path": "github.com/pkg/errors", "Version": "v0.8.0", "Update": {"Version": "v0.9.1"}}`
						c.Env = []string{"GO_WANT_HELPER_PROCESS=1", "HELPER_OUTPUT=" + jsonOutput}
					} else if name == "go" && len(arg) > 0 && arg[0] == "get" {
						if len(arg) > 1 {
							assert.Equal(t, "github.com/pkg/errors@v0.9.1", arg[1])
						}
						c.Env = []string{"GO_WANT_HELPER_PROCESS=1", "HELPER_OUTPUT=updated"}
					} else {
						c.Env = []string{"GO_WANT_HELPER_PROCESS=1", "HELPER_OUTPUT="}
					}
					return c
				}
			},
			args:       []string{"upgrade", "--all"},
			wantErr:    false,
			wantOutput: []string{"Applying updates...", "Updated github.com/pkg/errors to v0.9.1", "Verifying changes", "Tests passed!", "Upgrade complete and verified!"},
			upgradeAll: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := rootCmd
			resetFlags(cmd)
			upgradeAll = tc.upgradeAll

			tmpDir := t.TempDir()
			oldWd, _ := os.Getwd()
			os.Chdir(tmpDir)
			defer os.Chdir(oldWd)

			tc.setupFunc(t)

			originalExecCommand := execCommand
			execCommand = tc.execFunc(t)
			defer func() { execCommand = originalExecCommand }()

            originalExecuteShellCommand := executeShellCommand
            executeShellCommand = func(cmd string) (string, error) {
                if cmd == "go test ./..." {
                    return "PASS", nil
                }
                return "", nil
            }
            defer func() { executeShellCommand = originalExecuteShellCommand }()

			output, err := executeCommand(cmd, tc.args...)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			for _, wantOutput := range tc.wantOutput {
				assert.Contains(t, output, wantOutput)
			}
		})
	}
}

func TestRunUpgrade_WithCandidates_All_FailAndFix(t *testing.T) {
	cmd := rootCmd
	resetFlags(cmd)
	upgradeAll = true

	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	os.WriteFile("package.json", []byte(`{"dependencies": {"lodash": "4.17.15"}}`), 0644)
    // create a dummy file to be fixed
    os.WriteFile("index.js", []byte("console.log('hello')"), 0644)

	calls := 0
	originalExecCommand := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		c := helperCommandContext(context.Background(), name, arg...)

		if name == "npm" && len(arg) > 0 && arg[0] == "outdated" {
			jsonOutput := `{"lodash": {"current": "4.17.15", "latest": "4.17.21"}}`
            // make the exit code 1 to test npm outdated failure fallback
			c.Env = []string{"GO_WANT_HELPER_PROCESS=1", "HELPER_OUTPUT=" + jsonOutput, "HELPER_FAIL=1"}
		} else if name == "npm" && len(arg) > 0 && arg[0] == "install" {
            if len(arg) > 1 {
			    assert.Equal(t, "lodash@4.17.21", arg[1])
            }
			c.Env = []string{"GO_WANT_HELPER_PROCESS=1", "HELPER_OUTPUT=installed"}
		} else {
			c.Env = []string{"GO_WANT_HELPER_PROCESS=1", "HELPER_OUTPUT="}
		}
		return c
	}
	defer func() { execCommand = originalExecCommand }()

    originalExecuteShellCommand := executeShellCommand
	executeShellCommand = func(cmd string) (string, error) {
		if strings.HasPrefix(cmd, "npm test") {
			calls++
			if calls == 1 {
				// Fail first time
				return "FAIL", assert.AnError
			}
			// Pass second time
			return "PASS", nil
		}
		if cmd == "git diff" {
			return "diff", nil
		}
		return "", nil
	}
	defer func() { executeShellCommand = originalExecuteShellCommand }()

	// Mock Agent
	mockAgent := new(UpgradeMockAgent)
	mockAgent.On("Send", mock.Anything, mock.Anything).
		Return("<file path=\"index.js\">fixed code</file>", nil)

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Execute
	output, err := executeCommand(cmd, "upgrade", "--all")

	assert.NoError(t, err)
	assert.Contains(t, output, "Applying updates...")
	assert.Contains(t, output, "Updated lodash to 4.17.21")
	assert.Contains(t, output, "Tests failed. Asking AI to fix...")
	assert.Contains(t, output, "Fixed index.js")
	assert.Contains(t, output, "Upgrade complete and verified!")
	assert.Equal(t, 2, calls)

    // verify file written
    content, _ := os.ReadFile("index.js")
    assert.Equal(t, "fixed code\n", string(content))
}
