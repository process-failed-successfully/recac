package main

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTourCmd_GeneratePlan(t *testing.T) {
	// Setup Mock Agent (using MockAgent from tickets_test.go)
	mockAgent := new(MockAgent)

	// Expect the plan prompt
	planJSON := `{
		"steps": [
			{"path": "README.md", "summary": "Project documentation"},
			{"path": "main.go", "summary": "Entry point"}
		]
	}`
	// We use Anything for prompt matching to be robust against wording changes
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(planJSON, nil).Once()

	// Execute Command
	cmd := generateTourPlanCmd(mockAgent, ".")
	msg := cmd()

	// Assert Result
	loadedMsg, ok := msg.(tourLoadedMsg)
	assert.True(t, ok)
	assert.Len(t, loadedMsg.steps, 2)
	assert.Equal(t, "README.md", loadedMsg.steps[0].Path)
	assert.Equal(t, "Project documentation", loadedMsg.steps[0].Summary)
}

func TestTourModel_Update_LoadContent(t *testing.T) {
	// Setup Model with initialized components
	steps := []TourStep{
		{Path: "test.txt", Summary: "Test file"},
	}
	items := make([]list.Item, len(steps))
	for i, s := range steps {
		items[i] = s
	}

	m := TourModel{
		ready:    true,
		steps:    steps,
		list:     list.New(items, list.NewDefaultDelegate(), 0, 0),
		viewport: viewport.New(100, 50), // Initialize viewport too
		width:    100,
		height:   50,
	}

	// Create a dummy file for testing content loading
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.txt"
	_ = writeFileFunc(filePath, []byte("Hello World"), 0644)

    // Safer: Use absolute path in step.
    m.steps[0].Path = filePath

    // Execute
    // We are testing Update handler for contentLoadedMsg
    contentMsg := contentLoadedMsg{content: "File Content Loaded"}
    updatedModel, _ := m.Update(contentMsg)

    tm := updatedModel.(TourModel)
    assert.Equal(t, "File Content Loaded", tm.content)
    assert.Contains(t, tm.viewport.View(), "File Content Loaded")
}

func TestTourModel_Update_LoadExplanation(t *testing.T) {
	// Setup Model with initialized components
	steps := []TourStep{
		{Path: "test.txt", Summary: "Test file"},
	}
	items := make([]list.Item, len(steps))
	for i, s := range steps {
		items[i] = s
	}

	m := TourModel{
		ready:    true,
		steps:    steps,
		list:     list.New(items, list.NewDefaultDelegate(), 0, 0),
		viewport: viewport.New(100, 50),
		width:    100,
		height:   50,
	}

	expMsg := explanationLoadedMsg{explanation: "This is an explanation."}
	updatedModel, _ := m.Update(expMsg)

    tm := updatedModel.(TourModel)
    assert.Equal(t, "This is an explanation.", tm.explanation)
    assert.Contains(t, tm.viewport.View(), "This is an explanation.")
    assert.False(t, tm.loading)
}
