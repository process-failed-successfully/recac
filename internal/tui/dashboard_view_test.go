package tui

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDashboardModel_View_Quitting(t *testing.T) {
	model := DashboardModel{
		quitting: true,
	}

	view := model.View()
	assert.Contains(t, view, "Exiting dashboard...")
}

func TestDashboardModel_View_Error(t *testing.T) {
	err := errors.New("something went wrong")
	model := DashboardModel{
		err: err,
	}

	view := model.View()
	assert.Contains(t, view, "Error polling orchestrator")
	assert.Contains(t, view, "something went wrong")
	assert.Contains(t, view, "Press q to quit")
}
