package ui

import (
	"recac/internal/runner"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSessionManager for TUI tests
type MockPsSessionManager struct {
	mock.Mock
}

func (m *MockPsSessionManager) RenameSession(oldName, newName string) error {
	args := m.Called(oldName, newName)
	return args.Error(0)
}

func (m *MockPsSessionManager) DeleteSession(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockPsSessionManager) ArchiveSession(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func TestInteractivePsModel_Update_Navigation(t *testing.T) {
	sm := new(MockPsSessionManager)
	sessions := []runner.SessionState{{Name: "session-1"}, {Name: "session-2"}}
	model := NewInteractivePsModel(sm, sessions)

	// Test quit
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.Equal(t, tea.Quit(), cmd())

	// Test down
	m, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, "session-2", m.(InteractivePsModel).list.SelectedItem().(sessionItem).Name)
}

func TestInteractivePsModel_Update_Rename(t *testing.T) {
	sm := new(MockPsSessionManager)
	sessions := []runner.SessionState{{Name: "session-1"}}
	model := NewInteractivePsModel(sm, sessions)
	var m tea.Model = model
	var cmd tea.Cmd

	// 1. Press 'r' to trigger rename mode
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	assert.NotNil(t, cmd)

	// 2. Process the message to enter rename mode
	msg := cmd()
	m, _ = m.Update(msg)
	model = m.(InteractivePsModel)
	assert.Equal(t, modeRenaming, model.mode)
	assert.True(t, model.textInput.Focused())
	assert.Equal(t, "session-1", model.textInput.Value())

	// 3. Type new name (simulates user typing)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("-new")})
	model = m.(InteractivePsModel)
	assert.Equal(t, "session-1-new", model.textInput.Value())

	// 4. Press enter to confirm
	sm.On("RenameSession", "session-1", "session-1-new").Return(nil).Once()
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// 5. Process the command to rename, then process the success message
	msg = cmd()
	m, _ = m.Update(msg)
	model = m.(InteractivePsModel)
	assert.Equal(t, modeNavigating, model.mode)

	// 6. Verify mock was called and list item was updated
	sm.AssertExpectations(t)
	assert.Equal(t, "session-1-new", model.list.Items()[0].(sessionItem).Name)
}

func TestInteractivePsModel_Update_Delete(t *testing.T) {
	sm := new(MockPsSessionManager)
	sessions := []runner.SessionState{{Name: "session-1"}}
	var m tea.Model = NewInteractivePsModel(sm, sessions)
	var cmd tea.Cmd

	// 1. Press 'x' to trigger confirmation
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	msg := cmd()
	m, _ = m.Update(msg)
	model := m.(InteractivePsModel)
	assert.Equal(t, modeConfirmingDelete, model.mode)

	// 2. Press 'y' to confirm
	sm.On("DeleteSession", "session-1").Return(nil).Once()
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})

	// 3. Process the async delete command and resulting message
	msg = cmd()
	m, _ = m.Update(msg)
	model = m.(InteractivePsModel)
	assert.Equal(t, modeNavigating, model.mode)

	// 4. Verify mock was called and list is empty
	sm.AssertExpectations(t)
	assert.Empty(t, model.list.Items())
}

func TestInteractivePsModel_Update_Delete_Cancel(t *testing.T) {
	sm := new(MockPsSessionManager)
	sessions := []runner.SessionState{{Name: "session-1"}}
	var m tea.Model = NewInteractivePsModel(sm, sessions)
	var cmd tea.Cmd

	// 1. Press 'x' to trigger confirmation
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	msg := cmd()
	m, _ = m.Update(msg)
	model := m.(InteractivePsModel)
	assert.Equal(t, modeConfirmingDelete, model.mode)

	// 2. Press 'n' to cancel
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = m.(InteractivePsModel)
	assert.Equal(t, modeNavigating, model.mode)

	// 3. Verify mock was NOT called and item is still there
	sm.AssertNotCalled(t, "DeleteSession", "session-1")
	assert.Len(t, model.list.Items(), 1)
}
