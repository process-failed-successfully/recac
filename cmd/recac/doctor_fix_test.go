package main

import (
	"fmt"
	"os/exec"
	"recac/internal/ui"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/assert"
)

func TestDoctorFixCmd(t *testing.T) {
	origDiagnoseFunc := diagnoseFunc
	origAskOneFunc := askOneFunc
	origExecCommand := execCommand
	defer func() {
		diagnoseFunc = origDiagnoseFunc
		askOneFunc = origAskOneFunc
		execCommand = origExecCommand
	}()

	t.Run("AutoFix Git Identity", func(t *testing.T) {
		// Mock Diagnosis to return failure
		diagnoseFunc = func() []ui.Diagnostic {
			return []ui.Diagnostic{
				{
					Name:       "Git Identity",
					Status:     "FAIL",
					Message:    "Not configured",
					CanAutoFix: true,
					FixID:      "fix_git_identity",
				},
			}
		}

		// Mock Survey Input
		inputs := []string{"Test User", "test@example.com"}
		inputIndex := 0
		askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
			if inputIndex < len(inputs) {
				*(response.(*string)) = inputs[inputIndex]
				inputIndex++
			}
			return nil
		}

		// Mock Exec to capture calls
		var executedCmds []string
		execCommand = func(name string, arg ...string) *exec.Cmd {
			executedCmds = append(executedCmds, fmt.Sprintf("%s %v", name, arg))
			return exec.Command("true")
		}

		// Use executeCommand helper
		output, err := executeCommand(rootCmd, "doctor", "--fix")
		assert.NoError(t, err)

		assert.Contains(t, output, "Fixing Git Identity...")
		assert.Contains(t, output, "Successfully fixed Git Identity.")

		assert.Len(t, executedCmds, 2)
		assert.Contains(t, executedCmds[0], "git [config --global user.name Test User]")
		assert.Contains(t, executedCmds[1], "git [config --global user.email test@example.com]")
	})

	t.Run("Report Fixable Issues", func(t *testing.T) {
		diagnoseFunc = func() []ui.Diagnostic {
			return []ui.Diagnostic{
				{
					Name:       "Configuration",
					Status:     "FAIL",
					CanAutoFix: true,
				},
			}
		}

		output, err := executeCommand(rootCmd, "doctor")
		assert.NoError(t, err)
		assert.Contains(t, output, "1 issue(s) can be automatically fixed")
		assert.Contains(t, output, "Run 'recac doctor --fix'")
	})
}
