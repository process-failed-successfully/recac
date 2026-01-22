package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgent for Tour
type TourMockAgent struct {
	mock.Mock
}

func (m *TourMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *TourMockAgent) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	args := m.Called(ctx, prompt, callback)
	// Simulate streaming by calling callback
	callback(args.String(0))
	return args.String(0), args.Error(1)
}

func TestTourCmd(t *testing.T) {
	// 1. Setup Temp Dir
	tempDir := t.TempDir()

	mainContent := `package main

import "fmt"

func main() {
	fmt.Println("Start")
	helper()
}

func helper() {
	fmt.Println("Helper")
}
`
	err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(mainContent), 0644)
	assert.NoError(t, err)

	// 2. Mock Agent Factory
	originalAgentFactory := agentClientFactory
	defer func() { agentClientFactory = originalAgentFactory }()

	mockAgent := new(TourMockAgent)
	mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).Return("Function explanation here.", nil)

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// 3. Mock Survey (askOneFunc)
	originalAskOne := askOneFunc
	defer func() { askOneFunc = originalAskOne }()

	// Scenario: Start -> helper() -> Explain -> Quit
	// Steps:
	// 1. At main(): Select "Step into: helper()"
	// 2. At helper(): Select "Explain with AI"
	// 3. At helper() (after explanation): Select "Quit"

	step := 0
	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		fmt.Printf("Mock AskOneFunc called. Step: %d\n", step)
		selectPrompt, ok := p.(*survey.Select)
		if ok {
			// Select Mocking
			if step == 0 {
				// We expect to be in main(), looking for "Step into: helper()"
				fmt.Println("Looking for helper() option...")
				for _, opt := range selectPrompt.Options {
					if strings.Contains(opt, "helper()") {
						// Set response
						*(response.(*string)) = opt
						step++
						fmt.Println("Selected helper()")
						return nil
					}
				}
				// If not found, just Quit to avoid loop
				fmt.Println("helper() not found, Quitting")
				*(response.(*string)) = "Quit"
				return nil
			} else if step == 1 {
				// We expect to be in helper(), select "Explain with AI"
				fmt.Println("Selecting Explain with AI")
				*(response.(*string)) = "Explain with AI"
				step++
				return nil
			} else if step == 2 {
				// After explain, select "Quit"
				fmt.Println("Selecting Quit")
				*(response.(*string)) = "Quit"
				step++
				return nil
			} else {
				fmt.Println("Default Quit")
				*(response.(*string)) = "Quit"
				return nil
			}
		}

		// Input Mocking (Press Enter to continue)
		_, ok = p.(*survey.Input)
		if ok {
			fmt.Println("Input prompt (Enter to continue)")
			*(response.(*string)) = ""
			return nil
		}

		return fmt.Errorf("unexpected prompt type: %T", p)
	}

	// 4. Run Command via Root
	// We use rootCmd to ensure proper command hierarchy resolution
	rootCmd.SetArgs([]string{"tour", tempDir})

	// Capture Output
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err = rootCmd.Execute()
	assert.NoError(t, err)

	output := out.String()

	// 5. Verify Output
	assert.Contains(t, output, "Analyzing package")
	assert.Contains(t, output, "Current Function: main")
	assert.Contains(t, output, "Current Function: helper")
	assert.Contains(t, output, "Asking Agent...")
	assert.Contains(t, output, "Function explanation here.")
	assert.Contains(t, output, "Tour ended.")
}

func TestTourCmd_Methods(t *testing.T) {
	// 1. Setup Temp Dir
	tempDir := t.TempDir()

	mainContent := `package main

type Worker struct{}

func (w *Worker) DoWork() {
}

func main() {
	w := &Worker{}
	w.DoWork()
}
`
	err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(mainContent), 0644)
	assert.NoError(t, err)

	// 2. Mock Agent (not needed but factory must be set)
	originalAgentFactory := agentClientFactory
	defer func() { agentClientFactory = originalAgentFactory }()

	mockAgent := new(TourMockAgent)
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// 3. Mock Survey
	originalAskOne := askOneFunc
	defer func() { askOneFunc = originalAskOne }()

	step := 0
	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		selectPrompt, ok := p.(*survey.Select)
		if ok {
			if step == 0 {
				// Expect option for DoWork
				// The label should be "Step into: Worker.DoWork() (via DoWork)" or "w.DoWork" depending on parser
				for _, opt := range selectPrompt.Options {
					if strings.Contains(opt, "DoWork") {
						*(response.(*string)) = opt
						step++
						return nil
					}
				}
				*(response.(*string)) = "Quit"
				return nil
			} else {
				*(response.(*string)) = "Quit"
				return nil
			}
		}
		// Input prompt
		_, ok = p.(*survey.Input)
		if ok {
			*(response.(*string)) = ""
			return nil
		}
		return nil
	}

	// 4. Run via Root
	rootCmd.SetArgs([]string{"tour", tempDir})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err = rootCmd.Execute()
	assert.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Current Function: Worker.DoWork")
}
