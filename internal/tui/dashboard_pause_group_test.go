package tui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestPauseResumeGroupCmds(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/groups/my-group/pause" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/groups/my-group/resume" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/groups/bad-group/pause" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cmd := pauseGroupCmd(ts.URL, "my-group")
	msg := cmd()
	assert.IsType(t, actionMsg{}, msg)
	assert.Equal(t, "Concurrency group paused", msg.(actionMsg).Message)
	assert.Nil(t, msg.(actionMsg).Err)

	cmd = resumeGroupCmd(ts.URL, "my-group")
	msg = cmd()
	assert.IsType(t, actionMsg{}, msg)
	assert.Equal(t, "Concurrency group resumed", msg.(actionMsg).Message)
	assert.Nil(t, msg.(actionMsg).Err)

	cmd = pauseGroupCmd(ts.URL, "bad-group")
	msg = cmd()
	assert.IsType(t, actionMsg{}, msg)
	assert.NotNil(t, msg.(actionMsg).Err)

	cmd = pauseGroupCmd("http://invalid-url", "my-group")
	msg = cmd()
	assert.IsType(t, actionMsg{}, msg)
	assert.NotNil(t, msg.(actionMsg).Err)
}

func TestUpdatePauseGroupInput(t *testing.T) {
	m := DashboardModel{
		viewState:       viewPauseGroupInput,
		pauseGroupInput: textinput.New(),
		host:            "http://dummy",
	}

	// Test Esc key
	m2, cmd := m.updatePauseGroupInput(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Nil(t, cmd)
	assert.Equal(t, viewMain, m2.viewState)

	// Test Enter with empty value
	m.viewState = viewPauseGroupInput
	m2, cmd = m.updatePauseGroupInput(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd)
	assert.Equal(t, viewMain, m2.viewState)
	assert.EqualError(t, m2.err, "Concurrency group cannot be empty")

	// Test Enter with value
	m.viewState = viewPauseGroupInput
	m.pauseGroupInput.SetValue("test-group")
	m2, cmd = m.updatePauseGroupInput(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd)
	assert.Equal(t, viewMain, m2.viewState)
	assert.Nil(t, m2.err)
}

func TestUpdateResumeGroupInput(t *testing.T) {
	m := DashboardModel{
		viewState:        viewResumeGroupInput,
		resumeGroupInput: textinput.New(),
		host:             "http://dummy",
	}

	// Test Esc key
	m2, cmd := m.updateResumeGroupInput(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Nil(t, cmd)
	assert.Equal(t, viewMain, m2.viewState)

	// Test Enter with empty value
	m.viewState = viewResumeGroupInput
	m2, cmd = m.updateResumeGroupInput(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd)
	assert.Equal(t, viewMain, m2.viewState)
	assert.EqualError(t, m2.err, "Concurrency group cannot be empty")

	// Test Enter with value
	m.viewState = viewResumeGroupInput
	m.resumeGroupInput.SetValue("test-group")
	m2, cmd = m.updateResumeGroupInput(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd)
	assert.Equal(t, viewMain, m2.viewState)
	assert.Nil(t, m2.err)
}
