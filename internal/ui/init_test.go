package ui

import (
	"testing"

	"recac/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestBoardModel_Init(t *testing.T) {
	m := NewBoardModel(nil, nil, nil)
	assert.Nil(t, m.Init())
}

func TestBlameModel_Init(t *testing.T) {
	lines := []BlameLine{
		{LineNo: 1, Content: "code", SHA: "abc", Author: "me", Date: "now", Summary: "init"},
	}
	m := NewBlameModel(lines, nil, nil)
	assert.Nil(t, m.Init())
}

func TestMonitorDashboardModel_Init(t *testing.T) {
	callbacks := ActionCallbacks{
		GetSessions: func() ([]model.UnifiedSession, error) { return nil, nil },
	}
	m := NewMonitorDashboardModel(callbacks)
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestTopDashboardModel_Init(t *testing.T) {
	m := NewTopDashboardModel()
	cmd := m.Init()
	assert.NotNil(t, cmd)
}
