package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestConfigWizardModel_Update(t *testing.T) {
	m := NewConfigWizardModel()

	// Initial State: Provider Selection
	assert.Equal(t, ConfigStepProvider, m.step)

	// Select first item (Gemini)
	m.list.Select(0)
	msg := tea.KeyMsg{Type: tea.KeyEnter}

	updatedModel, _ := m.Update(msg)
	m = updatedModel.(ConfigWizardModel)

	// Step 2: API Key
	assert.Equal(t, ConfigStepAPIKey, m.step)
	assert.Equal(t, "gemini", m.Data.Provider)

	// Enter API Key
	m.textInput.SetValue("my-api-key")
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(ConfigWizardModel)

	// Step 3: Jira Enabled
	assert.Equal(t, ConfigStepJiraEnabled, m.step)
	assert.Equal(t, "my-api-key", m.Data.APIKey)

	// Say No to Jira
	m.textInput.SetValue("n")
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(ConfigWizardModel)

	// Should jump to Confirm
	assert.Equal(t, ConfigStepConfirm, m.step)
	assert.False(t, m.Data.JiraEnabled)

	// Confirm
	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(ConfigWizardModel)

	// Check if cmd is Quit. We can't compare functions, but we can verify state.
	assert.True(t, m.Data.Confirmed)
	// cmd should not be nil if it's returning Quit
	assert.NotNil(t, cmd)
}

func TestConfigWizardModel_JiraFlow(t *testing.T) {
	m := NewConfigWizardModel()

	// Fast forward to Jira Step
	m.step = ConfigStepJiraEnabled
	m.Data.Provider = "gemini"
	m.Data.APIKey = "key"

	msg := tea.KeyMsg{Type: tea.KeyEnter}

	// Say Yes to Jira
	m.textInput.SetValue("yes")
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(ConfigWizardModel)

	assert.Equal(t, ConfigStepJiraURL, m.step)
	assert.True(t, m.Data.JiraEnabled)

	// Enter URL
	m.textInput.SetValue("https://jira.example.com")
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(ConfigWizardModel)
	assert.Equal(t, "https://jira.example.com", m.Data.JiraURL)
	assert.Equal(t, ConfigStepJiraEmail, m.step)

	// Enter Email
	m.textInput.SetValue("user@example.com")
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(ConfigWizardModel)
	assert.Equal(t, "user@example.com", m.Data.JiraEmail)
	assert.Equal(t, ConfigStepJiraToken, m.step)

	// Enter Token
	m.textInput.SetValue("token123")
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(ConfigWizardModel)
	assert.Equal(t, "token123", m.Data.JiraToken)
	assert.Equal(t, ConfigStepConfirm, m.step)
}
